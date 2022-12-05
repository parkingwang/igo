package web

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/gin-gonic/gin"
)

type Router interface {
	// rpc模式路由方法
	// handler 支持rpc方法和gin.HandleFunc(为了支持gin中间件)
	Get(path string, handler ...any) Commenter
	Post(path string, handler ...any) Commenter
	Put(path string, handler ...any) Commenter
	Patch(path string, handler ...any) Commenter
	Delete(path string, handler ...any) Commenter
	Handle(method, path string, handler ...any) Commenter
	// 同gin
	Use(handler ...gin.HandlerFunc) Router
	Group(path string, handler ...gin.HandlerFunc) GroupCommenter
}

type Commenter interface {
	Comment(string)
}

type GroupCommenter interface {
	Commenter
	Router
}

type route struct {
	opt      *option
	basepath string
	r        gin.IRoutes
	isDir    bool
	info     *routeInfo
}

func (s *route) Get(path string, handler ...any) Commenter {
	return s.Handle(http.MethodGet, path, handler...)
}

func (s *route) Post(path string, handler ...any) Commenter {
	return s.Handle(http.MethodPost, path, handler...)
}

func (s *route) Put(path string, handler ...any) Commenter {
	return s.Handle(http.MethodPut, path, handler...)
}

func (s *route) Patch(path string, handler ...any) Commenter {
	return s.Handle(http.MethodPatch, path, handler...)
}

func (s *route) Delete(path string, handler ...any) Commenter {
	return s.Handle(http.MethodDelete, path, handler...)
}

func (s *route) Comment(c string) {
	if s.info != nil {
		s.info.comment = c
	}
}

func (s *route) Use(handler ...gin.HandlerFunc) Router {
	s2 := *s
	s2.r = s.r.Use(handler...)
	return &s2
}

func (s *route) Group(path string, handler ...gin.HandlerFunc) GroupCommenter {
	r := s.r.(gin.IRouter).Group(path, handler...)
	return &route{
		opt:      s.opt,
		r:        r,
		basepath: r.BasePath(),
		isDir:    true,
		info:     s.opt.routes.addGroup(r.BasePath()),
	}
}

func (s *route) Handle(method, path string, handler ...any) Commenter {
	if strings.Contains(path, "*") {
		panic("rpc handler not support *path")
	}
	hs := make([]gin.HandlerFunc, len(handler))
	var info *routeInfo
	for i, h := range handler {
		ginFunc, ok := h.(func(*gin.Context))
		if ok {
			hs[i] = ginFunc
		} else {
			if info != nil {
				panic("handle only support one rpc handler")
			}
			// 添加到路由信息表 为了自动生成doc
			info = s.opt.routes.addRoute(s.basepath, path, h, method)
			// 使用handleWarpf 转为gin.HandleFunc
			hs[i] = handleWarpf(s.opt)(h)
		}
	}
	s.r.Handle(method, path, hs...)
	if info != nil {
		return &route{info: info}
	}
	return nil
}

type routeInfo struct {
	isDir    bool
	basePath string
	path     string
	comment  string
	// handle only
	pcName  string
	method  string
	funType reflect.Value
	// dir only
	children Routes
}

type Routes []*routeInfo

func (r *Routes) addRoute(basepath, path string, h any, method string) *routeInfo {
	name := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	// ns := strings.Split(name, "/")
	info := &routeInfo{
		path:     path,
		basePath: basepath,
		// pcName:   ns[len(ns)-1],
		pcName:  name,
		method:  method,
		funType: reflect.ValueOf(h),
	}
	if basepath != "" {
		for k, v := range *r {
			if v.isDir && v.basePath == basepath {
				(*r)[k].children = append((*r)[k].children, info)
				return info
			}
		}
		return nil
	} else {
		*r = append(*r, info)
		return info
	}
}

func (r *Routes) addGroup(path string) *routeInfo {
	info := &routeInfo{
		isDir:    true,
		basePath: path,
		children: make([]*routeInfo, 0),
	}
	*r = append(*r, info)
	return info
}

func (r *Routes) echo() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	for _, v := range *r {
		if v.isDir {
			if len(v.children) == 0 {
				continue
			}
			fmt.Fprintf(w, "[router]├── %s\t\t\t%s\n", v.basePath, v.comment)
			for _, h := range v.children {
				fmt.Fprintf(w, "[router]│   └── %s\t%s\t%s\t%s\n", h.path, h.method, h.pcName, h.comment)
			}
		} else {
			fmt.Fprintf(w, "[router]├── %s\t%s\t%s\t%s\n", v.path, v.method, v.pcName, v.comment)
		}
	}
	w.Flush()
}

func SkipBindRequest() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Set("_igo_skip_bind", true)
	}
}
