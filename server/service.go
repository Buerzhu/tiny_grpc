package server

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/Buerzhu/tiny_grpc/codec"
)

// 服务列表，key为服务名，val为*service
var serviceMap sync.Map

type service struct {
	name    string                    //服务名称
	typ     reflect.Type              //服务对象的反射类型
	val     reflect.Value             //服务对象的反射值
	methods map[string]reflect.Method //服务对外提供的所有方法列表
}

type request struct {
	header     *codec.ReqHeader //请求的头部
	argv       reflect.Value    //请求的参数反射类型
	srv        *service         //请求所调用的服务
	methodName string
	method     reflect.Method //请求所调用的方法
}

type response struct {
	header *codec.RspHeader //请求的头部
	replyv reflect.Value    //返回值的反射类型
}

// 根据方法名动态调用服务提供的对应方法
func (s *service) call(methodName string, ctx context.Context, req interface{}, rsp interface{}) error {
	method, ok := s.methods[methodName]
	if !ok {
		return fmt.Errorf("invaild method name:%s", methodName)
	}
	rt := method.Func.Call([]reflect.Value{s.val, reflect.ValueOf(ctx), reflect.ValueOf(req), reflect.ValueOf(rsp)})
	if err := rt[0].Interface(); err != nil {
		return err.(error)
	}

	return nil
}
