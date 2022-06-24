package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Buerzhu/tiny_grpc/codec"
	"github.com/Buerzhu/tiny_grpc/config"
	"github.com/Buerzhu/tiny_grpc/naming"
	"github.com/Buerzhu/tiny_grpc/pool/connpool"
	log "github.com/golang/glog"

	"github.com/afex/hystrix-go/hystrix"
	"github.com/google/uuid"
)

type Client interface {
	Call(ctx context.Context, method string, req interface{}, rsp interface{}) error
}

type client struct {
	opt    *codec.Option
	path   string
	codec  codec.Codec
	addr   string
	conf   *config.ClientService
	header *codec.ReqHeader
	conn   net.Conn
}

// NewClientProxy 获取客户端代理句柄，name: 业务名/服务名
func NewClientProxy(ctx context.Context, name string, opts ...Option) Client {
	conf := config.GetClientConfig(name)
	opt := &codec.Option{
		MagicNumber: codec.MagicNumber,
		Version:     codec.Version,
		CodecType:   conf.SerializationType,
		Timeout:     time.Duration(conf.Timeout) * time.Millisecond,
	}

	// 用户传入的配置优先级高于框架配置
	for _, o := range opts {
		o(opt)
	}

	//全链路超时机制
	d, ok := ctx.Deadline()
	if ok {
		t := time.Until(d)
		if t < opt.Timeout {
			opt.Timeout = t
		}
	}

	// 基于traceID实现全链路请求跟踪
	id := uuid.NewString()
	v, ok := ctx.Value(codec.TraceID).(string)
	if ok {
		id = v
	}

	return &client{
		opt:  opt,
		path: name,
		header: &codec.ReqHeader{
			ID: id,
		},
		conf: conf,
	}
}

func (c *client) Call(ctx context.Context, method string, req interface{}, rsp interface{}) error {
	if c.opt.Timeout <= 0 {
		return errors.New("context timeout")
	}

	//设置超时时间
	ctx, cancel := context.WithTimeout(ctx, c.opt.Timeout)
	defer cancel()
	ctx = context.WithValue(ctx, codec.TraceID, c.header.ID)

	srvName, err := c.getServiceFromPath()
	if err != nil {
		return fmt.Errorf("client fail becasue of getServiceFromPath err:%+v", err)
	}
	c.header.ServiceMethod = srvName + "/" + method

	// 从名字服务获取服务端实际的ip地址
	addr, err := naming.GetNodeAddr("/" + c.path)
	if err != nil {
		return fmt.Errorf("client fail becasue of GetNodeAddr err:%+v", err)
	}
	c.addr = addr

	var conn net.Conn
	// 注意，这里连接池只支持tcp
	if c.conf.UseConnPool == true {
		conn, err = connpool.Get(ctx, addr, c.conf.NetWork)
		c.conn = conn
		c.opt.KeepAlive = true // 使用连接池意味着建立长连接

	} else {
		conn, err = c.initConn(ctx)
		c.conn = conn
	}
	if err != nil {
		return fmt.Errorf("client Call fail because client init conn fail.err:%+v", err)
	}

	//获取序列化方法
	codec, err := codec.GetCodec(c.opt.CodecType, conn)
	if err != nil {
		return fmt.Errorf("client fail becasue of codec.GetCodec err:%+v", err)
	}
	c.codec = codec

	finish := make(chan struct{})
	defer close(finish)
	if c.conf.UseHystrix == true { // 根据配置判断是否开启容灾
		return c.invokeWithHystrix(ctx, req, rsp)
	}

	go func() {
		err = c.invoke(ctx, req, rsp)
		if ctx.Err() == nil {
			finish <- struct{}{}
		}
	}()

	// 超时控制
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-finish:
		return err
	}
}

func (c *client) getServiceFromPath() (string, error) {
	if c.path == "" {
		return "", errors.New("empty service path")
	}

	index := strings.Index(c.path, "/")
	if index == -1 || index == 0 || index >= (len(c.path)-1) {
		log.Errorf("getServiceAndMethod err: invaild serviceMethod %s", c.path)
		return "", errors.New("invaild service path")
	}
	srvName := c.path[(index + 1):]
	return srvName, nil

}

func (c *client) invoke(ctx context.Context, req interface{}, rsp interface{}) error {
	// 如果是从连接池获取的连接，调用close会放到回收到连接池
	defer func() {
		c.codec.Close()
	}()

	// 写入Option，Option以uint64形式写入conn
	if err := writeOption(ctx, c.conn, c.opt); err != nil {
		if err != nil {
			return err
		}
	}

	// 写入请求
	err := c.codec.Write(c.header, req)
	if err != nil {
		return err
	}

	// 若写入超时则退出协程
	id := ctx.Value(codec.TraceID).(string)
	if err := isTimeout(ctx); err != nil {
		log.Errorf("id:%s,err:before ReadRspHeader time has ran out", id)
		return err
	}

	// 读取header
	rspHeader := &codec.RspHeader{}
	err = c.codec.ReadRspHeader(rspHeader)
	if err != nil {
		return fmt.Errorf("invoke fail because read rsp header err:%+v", err)
	}
	if rspHeader.Code != 0 {
		return fmt.Errorf("invoke fail because server return invalid rsp.code:%d,msg:%s",
			rspHeader.Code, rspHeader.Msg)
	}

	// 读取返回值
	err = c.codec.ReadBody(rsp)
	if err != nil {
		return fmt.Errorf("invoke fail because read rsp body err:%+v", err)
	}

	return nil
}

// 初始化连接
func (c *client) initConn(ctx context.Context) (net.Conn, error) {
	timeout := time.Duration(c.conf.Timeout) * time.Millisecond
	d, ok := ctx.Deadline()
	if ok {
		t := time.Until(d)
		if t < timeout {
			timeout = t
		}
	}
	conn, err := net.DialTimeout(c.conf.NetWork, c.addr, timeout)
	return conn, err
}

func (c *client) invokeWithHystrix(ctx context.Context, req interface{}, rsp interface{}) error {
	hystrix.ConfigureCommand(c.path, *transferHystrixConfig(c.conf.Hystrix))
	var err error
	finish := make(chan struct{})
	defer close(finish)
	// 注意这里是异步逻辑
	errChan := hystrix.Go(c.path,
		func() error {
			err = c.invoke(ctx, req, rsp)
			finish <- struct{}{}
			return err
		},
		func(err error) error {
			log.Errorf("service %s fail because of err:%+v", c.path, err)
			return err
		})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	case <-finish:
		return err
	}
}

func isTimeout(ctx context.Context) error {
	if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
		return ctx.Err()
	}
	return nil
}

// option长度固定为8字节，使用大端模式写入
// TODO:使用对象池优化性能
func writeOption(ctx context.Context, conn net.Conn, opt *codec.Option) error {
	var data uint64
	var keepAlive uint64
	if opt.KeepAlive {
		keepAlive = 1
	}
	magicNumber := uint64(opt.MagicNumber) << 32
	version := uint64(opt.Version) << 24
	keepAlive <<= 20
	typ := uint64(codec.CodecToIntMap[opt.CodecType]) << 16
	timeout := uint16(opt.Timeout / time.Millisecond)
	data = magicNumber | version | keepAlive | typ | uint64(timeout)

	var pkg = new(bytes.Buffer)
	// 大端模式写入
	err := binary.Write(pkg, binary.BigEndian, data)
	if err != nil {
		return fmt.Errorf("writeOption fail because of binary.Write err:%+v", err)
	}

	_, err = conn.Write(pkg.Bytes())
	if err != nil {
		return fmt.Errorf("writeOption fail because of conn.Write err:%+v", err)
	}

	return nil
}

func transferHystrixConfig(conf *config.HystrixConfig) *hystrix.CommandConfig {
	return &hystrix.CommandConfig{
		Timeout:                conf.Timeout,
		MaxConcurrentRequests:  conf.MaxConcurrentRequests,
		RequestVolumeThreshold: conf.RequestVolumeThreshold,
		SleepWindow:            conf.SleepWindow,
		ErrorPercentThreshold:  conf.ErrorPercentThreshold,
	}
}
