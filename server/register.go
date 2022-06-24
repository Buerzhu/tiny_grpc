package server

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"reflect"

	log "github.com/golang/glog"
)

// 检查参数是否为可导出的
func isExportedOrBuiltinType(t reflect.Type) bool {
	// Name是字段的名字，PkgPath是非导出字段的包路径，对导出字段该字段为""
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

func checkMethodType(t reflect.Type) error {
	context.Background()
	// 检查入参与出参的数量是否正确，入参的第0个参数为自身self
	if t.NumIn() != 4 || t.NumOut() != 1 {
		return errors.New("wrong param num")
	}
	// 检查函数第一个参数是否为context类型
	if t.In(1).Name() != "Context" {
		return errors.New("first param is not context type")
	}
	// 函数第二、第三个参数分别为req和rsp,检查req和rsp参数是否为指针类型
	if t.In(2).Kind() != reflect.Ptr || t.In(3).Kind() != reflect.Ptr {
		return errors.New("input param is not pointer type")
	}
	// 检查出参是否为error类型
	if t.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
		return errors.New("out param is not error type")
	}
	// 检查参数是否为可导出的
	if !isExportedOrBuiltinType(t.In(2)) || !isExportedOrBuiltinType(t.In(3)) {
		return errors.New("input param is not exportable")
	}
	return nil
}

// 注册服务以及服务对外提供的接口
func Register(srv interface{}) error {
	// 接口值不能为空指针
	if reflect.ValueOf(srv).IsNil() == true {
		return errors.New("Register fail because service handler is empty pointer")
	}
	newService := &service{
		val:     reflect.ValueOf(srv),
		typ:     reflect.TypeOf(srv),
		name:    reflect.Indirect(reflect.ValueOf(srv)).Type().Name(),
		methods: make(map[string]reflect.Method),
	}

	if !ast.IsExported(newService.name) {
		log.Errorf("Register fail because %s is not a valid service\n", newService.name)
		return fmt.Errorf("invaild service")
	}
	for i := 0; i < newService.typ.NumMethod(); i++ {
		method := newService.typ.Method(i)
		if err := checkMethodType(method.Type); err != nil {
			log.Errorf("Register fail because %s is not a valid method\n", method.Name)
			return fmt.Errorf("invaild service method. err:%+v", err)
		}
		newService.methods[method.Name] = method
	}
	serviceMap.Store(newService.name, newService)

	return nil
}
