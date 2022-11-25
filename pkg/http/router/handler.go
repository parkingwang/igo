package router

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"

	"github.com/gin-gonic/gin"
)

type Router interface {
	// 添加备注
	Comment(string) Router
	// rpc模式路由方法
	// handler 支持rpc方法和gin.HandleFunc(为了支持gin中间件)
	Get(path string, handler ...any)
	Post(path string, handler ...any)
	Put(path string, handler ...any)
	Patch(path string, handler ...any)
	Delete(path string, handler ...any)
	Handle(method, path string, handler ...any)
	// 同gin
	Use(handler ...gin.HandlerFunc)
	Group(path string, handler ...gin.HandlerFunc) Router
}

type route struct {
	opt      *option
	basepath string
	describe string
	r        gin.IRoutes
}

func (s *route) Get(path string, handler ...any) {
	s.Handle(http.MethodGet, path, handler...)
}

func (s *route) Post(path string, handler ...any) {
	s.Handle(http.MethodPost, path, handler...)
}

func (s *route) Put(path string, handler ...any) {
	s.Handle(http.MethodPut, path, handler...)
}

func (s *route) Patch(path string, handler ...any) {
	s.Handle(http.MethodPatch, path, handler...)
}

func (s *route) Delete(path string, handler ...any) {
	s.Handle(http.MethodDelete, path, handler...)
}

func (s *route) Comment(v string) Router {
	s2 := *s
	s2.describe = v
	return &s2
}

func (s *route) Use(handler ...gin.HandlerFunc) {
	s.r.Use(handler...)
}

func (s *route) Group(path string, handler ...gin.HandlerFunc) Router {
	r := s.r.(gin.IRouter).Group(path, handler...)
	return &route{opt: s.opt, r: r, basepath: r.BasePath()}
}

func (s *route) Handle(method, path string, handler ...any) {
	hs := make([]gin.HandlerFunc, len(handler))
	for i, h := range handler {
		x, ok := h.(func(*gin.Context))
		if ok {
			hs[i] = x
		} else {
			n := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
			fmt.Println(method, s.basepath+path, n, s.describe)
			hs[i] = handleWarpf(s.opt)(h)
		}
	}
	s.r.Handle(method, path, hs...)
}
