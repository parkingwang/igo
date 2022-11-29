package web

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/parkingwang/igo/pkg/http/web/oas"
	"golang.org/x/tools/go/packages"
)

type ApiDocOption struct {
	RoutePath string // 文档访问的路由
	Descripe  string // 接口描述
}

func (r *Routes) deep(rs []routeInfo, replacePkg func(string) string, maps map[string]string) {
	for k, route := range rs {
		if route.isDir {
			r.deep(route.children, replacePkg, maps)
			continue
		}
		i := strings.LastIndex(route.pcName, ".")
		rs[k].pcName = replacePkg(route.pcName[:i]) + route.pcName[i:]
		maps[replacePkg(route.pcName[:i])+route.pcName[i:]] = replacePkg(route.pcName[:i])
	}
}

func (r *Routes) ToDoc() {
	m := make(map[string]string)
	x, _ := debug.ReadBuildInfo()
	r.deep(*r, func(s string) string {
		if s == "main" {
			return x.Path
		}
		return s
	}, m)

	pkgnames := make(map[string]struct{})

	for _, v := range m {
		pkgnames[v] = struct{}{}
	}

	patterns := make([]string, 0)
	for k := range pkgnames {
		patterns = append(patterns, k)
	}

	cfg := &packages.Config{Fset: token.NewFileSet(), Mode: packages.NeedTypes | packages.NeedSyntax}
	pkgs, err := packages.Load(cfg, patterns...)

	if err != nil {
		fmt.Println(err)
		return
	}
	// 保存函数对应的注释
	funcComments := map[string]string{}
	for _, pkg := range pkgs {
		for _, s := range pkg.Syntax {
			for name, obj := range s.Scope.Objects {
				switch obj.Kind {
				case ast.Fun:
					fullname := pkg.ID + "." + name
					if _, ok := m[fullname]; ok {
						funcComments[fullname] = obj.Decl.(*ast.FuncDecl).Doc.Text()
					}
				}
			}
		}
	}

	spec := oas.NewOAS()
	spec.Info.Title = "demo"
	spec.Info.Description = "这里是一个描述看看从哪里后去合适"
	spec.Info.Version = "0.0.1"
	// spec.Components.Schema["responseError"] = oas.Generate(reflect.ValueOf(nil))

	paths := make(map[string]map[string]any)
	for _, route := range *r {
		if route.isDir {
			spec.Tags = append(spec.Tags, strings.TrimLeft(route.basePath, "/"))
			for _, ru := range route.children {
				toConveterRequest(paths, ru, funcComments[ru.pcName])
			}
		} else {
			toConveterRequest(paths, route, funcComments[route.pcName])
		}
	}
	spec.Paths = paths
	if err := json.NewEncoder(os.Stderr).Encode(spec); err != nil {
		fmt.Println(err)
	}
	runtime.GC()
}

const defaultContentType = "application/json"

var reqTypeEmpty = reflect.TypeOf(Empty{})

func toConveterRequest(root map[string]map[string]any, route routeInfo, comment string) {
	// 将gin的 :xx *xx 替换为openapi的 {xx}
	path := route.basePath + route.path
	ps := strings.Split(path, "/")
	for i, p := range ps {
		if strings.HasPrefix(p, ":") || strings.HasPrefix(p, "*") {
			ps[i] = "{" + p[1:] + "}"
		}
	}
	path = strings.Join(ps, "/")
	w, ok := root[path]
	if !ok {
		w = make(map[string]any)
	}

	// tag 按路由分组自动聚合
	var tags []string
	if route.basePath != "" {
		tags = []string{strings.TrimLeft(route.basePath, "/")}
	}

	var req map[string]oas.Schema
	var responseSchema map[string]oas.Schema
	tp := route.funType.Type()
	in := tp.In(1).Elem()
	// 请求参数跳过web.Empty对象
	if in != reqTypeEmpty {
		req = oas.Generate(reflect.New(in))
	}
	if tp.NumOut() == 2 {
		out := tp.Out(0).Elem()
		responseSchema = oas.Generate(reflect.New(out))
	}

	rp := oas.OASRequest{
		Tags:      tags,
		Summary:   comment,
		Responses: gin.H{"200": gin.H{"description": "成功 success"}},
	}

	if req != nil {
		rp.RequestBody = gin.H{"content": gin.H{defaultContentType: req}}
	}

	if responseSchema != nil {
		rp.Responses["200"].(gin.H)["content"] = gin.H{defaultContentType: responseSchema}
	}

	w[strings.ToLower(route.method)] = rp
	root[path] = w
}
