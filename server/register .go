package server

import (
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

func checkMethodType(t reflect.Type) bool {
	// 检查入参与出参的数量是否正确，入参的第一个参数为自身self
	if t.NumIn() != 3 || t.NumOut() != 1 {
		return false
	}
	// 检查出参是否为error类型
	if t.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
		return false
	}
	// 检查参数是否为可导出的
	return  isExportedOrBuiltinType(t.In(1)) && isExportedOrBuiltinType(t.In(2))
}

// 注册服务以及服务对外提供的接口
func Register(srv interface{}) error {
	newService := &service{
		val: reflect.ValueOf(srv),
		typ: reflect.TypeOf(srv),
		name: reflect.Indirect(reflect.ValueOf(srv)).Type().Name(),
		methods: make(map[string]reflect.Method),
	}
	
	if !ast.IsExported(newService.name) {
		log.Errorf("Register fail because %s is not a valid service", newService.name)
		return fmt.Errorf("invaild service")
	}
	for i := 0; i < newService.typ.NumMethod(); i++ {
		method := newService.typ.Method(i)
		if !checkMethodType(method.Type) {
			log.Errorf("Register fail because %s is not a valid method", method.Name)
			return fmt.Errorf("invaild service method")
		}
		newService.methods[method.Name]=method
	}
	serviceMap.Store(newService.name, newService)

	return nil

}