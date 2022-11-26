package web

import "reflect"

type ApiDocOption struct {
	RoutePath string // 文档访问的路由
	Descripe  string // 接口描述
}

type routeInfo struct {
	isDir   bool
	path    string
	comment string
	// handle only
	pcName  string
	method  string
	funType reflect.Type
	// dir only
	children []routeInfo
}
