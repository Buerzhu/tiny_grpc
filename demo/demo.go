package main

import (
	"context"
	"flag"
	"sync"
	"time"

	"github.com/Buerzhu/tiny_grpc/client"
	"github.com/Buerzhu/tiny_grpc/config"
	"github.com/Buerzhu/tiny_grpc/server"

	log "github.com/golang/glog"
)

var w sync.WaitGroup

func main() {
	flag.Parse()
	defer log.Flush()

	config.Init() // 加载配置

	go startServer()            //启动服务端
	time.Sleep(time.Second * 2) // 等待服务端启动完成
	startClient()               // 开始调用

}

type Service struct{}

type Req struct {
	A int
	B int
}

type Rsp struct {
	C int
}

func (s *Service) Sum(ctx context.Context, req *Req, rsp *Rsp) error {
	rsp.C = req.A + req.B
	return nil
}

func startServer() {
	// 注册服务
	t := &Service{}
	err := server.Register(t)
	if err != nil {
		log.Fatal("server.Register:%+v", err)
	}

	// 开始对外提供服务
	service, err := server.NewServer("App/Service")
	if err != nil {
		log.Fatal("server.NewServer:%v", err)
	}
	service.Serve()
}

func startClient() {
	req := &Req{A: 1, B: 2}
	rsp := &Rsp{}

	ctx := context.Background()
	s := client.NewClientProxy(ctx, "App/Service")
	now := time.Now()
	err := s.Call(ctx, "Sum", req, rsp)
	if err != nil {
		log.Errorf("call err:%+v", err)
	}
	log.Infof("req:%+v,rsp:%+v, use time:%dms\n", req, rsp, time.Now().Sub(now)/time.Millisecond)
}
