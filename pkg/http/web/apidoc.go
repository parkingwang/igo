package web

import (
	_ "embed"
	"go/ast"
	"go/token"
	"net/http"
	"reflect"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/parkingwang/igo/pkg/http/web/oas"
	"golang.org/x/tools/go/packages"
)

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

//go:embed oas/swagger-ui.html
var swaggerUIData []byte

func (r *Routes) ToDoc(info oas.DocInfo) (*oas.Spec, error) {
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
		return nil, err
	}
	// 保存函数对应的注释
	funcComments := map[string]oas.RequestComment{}
	for _, pkg := range pkgs {
		for _, s := range pkg.Syntax {
			for name, obj := range s.Scope.Objects {
				switch obj.Kind {
				case ast.Fun:
					fullname := pkg.ID + "." + name
					if _, ok := m[fullname]; ok {
						doc := obj.Decl.(*ast.FuncDecl).Doc.Text()
						ps := strings.SplitN(doc, "\n", 2)
						if len(ps) == 1 {
							ps = append(ps, "")
						}
						funcComments[fullname] = oas.RequestComment{
							Summary:     ps[0],
							Description: ps[1],
						}
					}
				}
			}
		}
	}

	spec := oas.NewSpec()
	spec.Info = info

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
	return spec, nil
}

var contentTypes = map[string]string{
	"json":      binding.MIMEJSON,
	"form":      binding.MIMEPOSTForm,
	"form-data": binding.MIMEMultipartPOSTForm,
}

var reqTypeEmpty = reflect.TypeOf(Empty{})

func toConveterRequest(root map[string]map[string]any, route routeInfo, comment oas.RequestComment) {
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

	rp := oas.Request{
		Tags:           tags,
		RequestComment: comment,
		OperationID:    createOperationID(route),
		Responses: map[string]oas.Body{"200": {
			Description: "Successful operation",
		}},
	}

	tp := route.funType.Type()
	in := tp.In(1).Elem()
	// 请求参数跳过web.Empty对象
	if in != reqTypeEmpty {
		parameters, bodytypes := toConveterParameters(route, in)
		if len(parameters) > 0 {
			rp.Parameters = parameters
		}
		if len(bodytypes) > 0 {
			body := &oas.Body{
				Required: true,
				Content:  make(map[string]oas.Schema),
			}
			for _, tag := range bodytypes {
				body.Content[contentTypes[tag]] = oas.Generate(reflect.New(in), tag)["schema"]
			}
			rp.RequestBody = body
		}
	}
	if tp.NumOut() == 2 {
		out := tp.Out(0).Elem()
		const responseTag = "json"
		rp.Responses["200"] = oas.Body{
			Description: "Successful operation",
			Content: map[string]oas.Schema{
				contentTypes[responseTag]: oas.Generate(reflect.New(out), responseTag)["schema"],
			},
		}
	}

	w[strings.ToLower(route.method)] = rp
	root[path] = w
}

func toConveterParameters(route routeInfo, in reflect.Type) ([]any, []string) {
	bodyType := map[string]struct{}{}
	var list []any
	for i := 0; i < in.NumField(); i++ {
		field := in.Field(i)
		item := map[string]any{
			"description": field.Tag.Get("comment"),
			"required":    (strings.Split(field.Tag.Get("binding"), ","))[0] == "required",
		}
		if v, ok := field.Tag.Lookup("header"); ok {
			item["name"] = v
			item["in"] = "header"
			item["schema"] = oas.Generate(reflect.New(field.Type), "header")
			list = append(list, item)
		}
		if v, ok := field.Tag.Lookup("uri"); ok {
			item["name"] = v
			item["in"] = "path"
			item["schema"] = oas.Generate(reflect.New(field.Type), "uri")
			list = append(list, item)
		}
		if v, ok := field.Tag.Lookup("form"); ok {
			// 非get 方法 form 会解析为表单
			if route.method == http.MethodGet {
				item["name"] = v
				item["in"] = "query"
				item["schema"] = oas.Generate(reflect.New(field.Type), "form")
				list = append(list, item)
			} else {
				bodyType["form"] = struct{}{}

			}
		}

		if _, ok := field.Tag.Lookup("json"); ok {
			bodyType["json"] = struct{}{}
		}
	}
	bodyTypes := []string{}
	for k := range bodyType {
		bodyTypes = append(bodyTypes, k)
	}
	return list, bodyTypes
}

func createOperationID(r routeInfo) string {
	path := strings.ReplaceAll(strings.ReplaceAll(r.basePath+r.path, ":", ""), "*", "")
	ps := strings.Split(path, "/")
	for i, v := range ps {
		if v != "" {
			ps[i] = strings.ToUpper(v[0:1]) + strings.ToLower(v[1:])
		}
	}
	return strings.ToLower(r.method) + strings.Join(ps, "") + "OperationId"
}
