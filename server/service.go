package server

import (
	"fmt"
	"reflect"
	"sync"
)

// 服务列表，key为服务名，val为*service
var serviceMap sync.Map

type service struct {
	name    string                    //服务名称
	typ     reflect.Type              //服务对象的反射类型
	val     reflect.Value             //服务对象的反射值
	methods map[string]reflect.Method //服务对外提供的所有方法列表
}

// 根据方法名动态调用服务提供的对应方法
func (s *service) call(methodName string, req interface{}, rsp interface{}) error {
	method, ok := s.methods[methodName]
	if !ok {
		return fmt.Errorf("invaild method name:%s", methodName)
	}
	rt := method.Func.Call([]reflect.Value{s.val, reflect.ValueOf(req), reflect.ValueOf(rsp)})
	if err := rt[0].Interface(); err != nil {
		return err.(error)
	}

	return nil
}
