package server

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/Buerzhu/tiny_grpc/codec"
	"github.com/Buerzhu/tiny_grpc/config"
	"github.com/Buerzhu/tiny_grpc/constant"
	"github.com/Buerzhu/tiny_grpc/naming"
	"github.com/Buerzhu/tiny_grpc/pool/workpool"

	log "github.com/golang/glog"
)

type Server interface {
	Serve()
}

type server struct {
	name     string
	conf     *config.ServerService
	listener *net.TCPListener
}

func NewServer(name string) (Server, error) {
	service := &server{name: name}
	conf := config.GetServerConfig()
	service.conf = conf
	localAddress, err := net.ResolveTCPAddr("tcp", ":0")
	if err != nil {
		return nil, fmt.Errorf("NewServer fail because of net.ResolveTCPAddr err:%+v", err)
	}
	listener, err := net.ListenTCP(conf.NetWork, localAddress)
	if err != nil {
		return nil, fmt.Errorf("NewServer fail because of net.ListenTCP err:%+v", err)
	}
	service.listener = listener

	err = naming.Register(name, listener.Addr().String())
	if err != nil {
		return nil, fmt.Errorf("NewServer fail because of naming.Register err:%+v", err)
	}
	return service, nil
}

// 目前只支持tcp
func (s *server) Serve() {
	log.Infof("%s start serve...", s.listener.Addr().String())
	for {
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			log.Errorf("listener accept fail because of err:%+v", err)
			continue
		}

		s.handleAsync(conn, time.Millisecond*time.Duration(s.conf.Timeout))
	}
}

func (s *server) handleAsync(conn *net.TCPConn, timeout time.Duration) {
	if s.conf.UseWorkPool == true {
		err := workpool.SubmitTask(func() error {
			s.handleConn(conn, timeout)
			return nil
		})
		if err != nil {
			log.Errorf("handleAsync fail because of workpool.SubmitTask err:%+v", err)
		}
	} else {
		go func() {
			s.handleConn(conn, timeout)
		}()
	}
}

// 在这里处理短连接和长连接的逻辑
func (s *server) handleConn(conn *net.TCPConn, timeout time.Duration) {
	defer func() {
		conn.Close()
	}()

	keepAlive, err := s.handle(conn, timeout)
	if err != nil {
		log.Infof("handleConn fail because of handle err:%+v\n", err)
		return
	}

	if !keepAlive {
		return
	}

	// 如果是长连接，则开始监听连接，并定时探测客户端心跳
	err = conn.SetKeepAlivePeriod(time.Second * 60)
	if err != nil {
		log.Infof("handleConn fail because of conn.SetKeepAlivePeriod err:%+v\n", err)
		return
	}

	for {
		_, err := s.handle(conn, timeout)
		if err != nil {
			log.Infof("handleConn fail because of handle err:%+v\n", err)
			return
		}
	}
}

func (s *server) handle(conn *net.TCPConn, timeout time.Duration) (bool, error) {
	// 解析请求的opt
	opt, err := parseOption(conn)
	if err != nil {
		log.Errorf("handle fail because of parseOption err:%+v", err)
		return false, errors.New("parseOption err")
	}

	// 超时控制
	if opt.Timeout < timeout {
		timeout = opt.Timeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	finish := make(chan struct{})
	defer close(finish)
	f := func() error {
		err = deal(ctx, opt, conn)
		if ctx.Err() == nil {
			finish <- struct{}{}
		}
		return err
	}
	if s.conf.UseWorkPool {
		workpool.SubmitTask(f)
	} else {
		f()
	}

	// 超时控制
	select {
	case <-ctx.Done():
		return opt.KeepAlive, ctx.Err()
	case <-finish:
		return opt.KeepAlive, err
	}
}

func deal(ctx context.Context, opt *codec.Option, conn *net.TCPConn) error {
	// 根据序列化方式获取解析器
	newCodecFunc := codec.CodecMap[opt.CodecType]
	codecc := newCodecFunc(conn)

	// 解析请求头和请求体
	rsp := &response{header: &codec.RspHeader{}}
	req, code, err := parseRequest(codecc)
	if err != nil {
		rsp.header.Code = code
		rsp.header.Msg = err.Error()
		log.Errorf("handle fail because of parseRequest err:%+v", err)
		sendResponse(codecc, rsp.header, "")
		return errors.New("parseRequest err")
	}

	if err := isTimeout(ctx, codecc, rsp); err != nil {
		return err
	}

	// 发起调用
	ctx = context.WithValue(ctx, codec.TraceID, req.header.ID)
	rsp.replyv = reflect.New(req.method.Type.In(3).Elem()) //method的第三个入参为rsp
	err = req.srv.call(req.methodName, ctx, req.argv.Interface(), rsp.replyv.Interface())
	if err != nil {
		log.Errorf("method %s call err: %+v", req.methodName, err)
		rsp.header.Code = constant.INTERNAL_ERR
		rsp.header.Msg = err.Error()
		sendResponse(codecc, rsp.header, "")
		return errors.New("method call err")
	}

	// 向客户端返回响应
	err = sendResponse(codecc, rsp.header, rsp.replyv.Interface())
	if err != nil {
		log.Errorf("hander fail becase of sendResponse err:%+v", err)
		return errors.New("response write err")
	}
	return nil
}

func sendResponse(codecc codec.Codec, header *codec.RspHeader, body interface{}) error {
	// 向客户端返回响应
	err := codecc.Write(header, body)
	if err != nil {
		log.Errorf("hander err: codecc write err:%+v", err)
		return errors.New("response write err")
	}
	return nil
}

func parseOption(conn *net.TCPConn) (*codec.Option, error) {
	opt, err := readOption(conn)
	if err != nil {
		log.Errorf("parseOption err:%+v", err)
		return nil, err
	}

	if opt.MagicNumber != codec.MagicNumber {
		log.Errorf("parseOption err: no rpc request, wrong magic number:%d", opt.MagicNumber)
		return nil, errors.New("no rpc request")
	}

	_, ok := codec.CodecMap[opt.CodecType]
	if !ok {
		log.Errorf("parseOption err: invalid codec type:%d", opt.CodecType)
		return nil, errors.New("invalid codec type")
	}
	return opt, nil
}

// option长度固定为8字节
func readOption(conn io.ReadWriteCloser) (*codec.Option, error) {
	data := make([]byte, 8)
	var n int
	var err error

	// 当keepalive探测客户端心跳断开或客户端关闭连接时报错误退出循环
	for n < 8 {
		n, err = conn.Read(data)
		if err != nil {
			return nil, fmt.Errorf("readOption fail because of conn.Read err:%+v", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("readOption fail because of conn.Read err:%+v", err)
	}
	head := binary.BigEndian.Uint64(data)
	magicNumber := uint32((head & 0xffffffff00000000) >> 32)
	version := uint8((head & 0x00000000ff000000) >> 24)
	keepAlive := uint8((head & 0x0000000000100000) >> 20)
	typ := uint8((head & 0x00000000000f0000) >> 16)
	timeout := uint16(head & 0x000000000000ffff)
	opt := &codec.Option{
		MagicNumber: magicNumber,
		Version:     version,
		KeepAlive:   keepAlive == 1,
		CodecType:   codec.CodecToStringMap[typ],
		Timeout:     time.Duration(timeout) * time.Millisecond,
	}

	return opt, nil
}

func parseRequest(codecc codec.Codec) (*request, int, error) {
	req := &request{header: &codec.ReqHeader{}}
	// 获取请求头
	err := codecc.ReadReqHeader(req.header)
	if err != nil {
		log.Errorf("parseRequest fail because read reqHeader err:%+v", err)
		return nil, constant.DECODE_ERR, errors.New(fmt.Sprintf("parseRequest err:%+v", err))
	}

	srv, method, err := getServiceAndMethod(req.header.ServiceMethod)
	if err != nil {
		log.Errorf("parseRequest because getServiceAndMethod err:%+v", err)
		return nil, constant.INVAILD_PARAM, errors.New(fmt.Sprintf("parseRequest err:%+v", err))
	}
	req.srv = srv
	req.method = method
	req.methodName = method.Name
	req.argv = reflect.New(method.Type.In(2).Elem()) // method第二个参数为req

	// 获取请求体
	if err = codecc.ReadBody(req.argv.Interface()); err != nil {
		log.Errorf("parseRequest because read body err:%+v", err)
		return nil, constant.DECODE_ERR, errors.New(fmt.Sprintf("parseRequest err:%+v", err))
	}

	return req, 0, nil
}

func getServiceAndMethod(serviceMethod string) (*service, reflect.Method, error) {
	index := strings.Index(serviceMethod, "/")
	if index == -1 || index == 0 || index >= (len(serviceMethod)-1) {
		log.Errorf("getServiceAndMethod err: invaild serviceMethod %s", serviceMethod)
		return nil, reflect.Method{}, errors.New("invaild serviceMethod")
	}

	srvName := serviceMethod[:index]
	methodName := serviceMethod[(index + 1):]
	srv, ok := serviceMap.Load(srvName)

	if !ok {
		log.Errorf("getServiceAndMethod err: service %s not found", srvName)
		return nil, reflect.Method{}, errors.New("invaild serviceMethod")
	}

	method, ok := srv.(*service).methods[methodName]
	if !ok {
		log.Errorf("getServiceAndMethod err: method %s not found", method)
		return nil, reflect.Method{}, errors.New("invaild serviceMethod")
	}

	return srv.(*service), method, nil
}

func isTimeout(ctx context.Context, codecc codec.Codec, rsp *response) error {
	if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
		rsp.header.Code = constant.TIMEOUT
		rsp.header.Msg = ctx.Err().Error()
		sendResponse(codecc, rsp.header, "")
		return ctx.Err()
	}
	return nil
}
