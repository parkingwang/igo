package web

import (
	_ "embed"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/parkingwang/igo/pkg/http/web/oas"
)

//go:embed oas/swagger-ui.html
var swaggerUIData []byte

func (r *Routes) ToDoc(info oas.DocInfo) (*oas.Spec, error) {

	spec := oas.NewSpec()
	spec.Info = info

	// spec.Components.Schema["responseError"] = oas.Generate(reflect.ValueOf(nil))

	paths := make(map[string]map[string]any)
	for _, route := range *r {
		if route.isDir {
			spec.Tags = append(spec.Tags, oas.Tag{
				Name:        strings.TrimLeft(route.basePath, "/"),
				Description: route.comment,
			})
			for _, ru := range route.children {
				toConveterRequest(paths, *ru)
			}
		} else {
			toConveterRequest(paths, *route)
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

func toConveterRequest(root map[string]map[string]any, route routeInfo) {
	// 将gin的 :xx 替换为openapi的 {xx}
	path := route.basePath + route.path
	ps := strings.Split(path, "/")
	for i, p := range ps {
		if strings.HasPrefix(p, ":") {
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
		Tags: tags,
		RequestComment: oas.RequestComment{
			Summary: route.comment,
		},
		OperationID: createOperationID(route),
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
			fmt.Println(tp.Name(), parameters)

			rp.Parameters = parameters
		}
		if len(bodytypes) > 0 {
			body := &oas.Body{
				Required: true,
				Content:  make(map[string]map[string]oas.Schema),
			}
			for _, tag := range bodytypes {
				body.Content[contentTypes[tag]] = oas.Generate(reflect.New(in), tag)
			}
			rp.RequestBody = body
		}
	}
	if tp.NumOut() == 2 {
		out := tp.Out(0).Elem()
		const responseTag = "json"
		rp.Responses["200"] = oas.Body{
			Description: "Successful operation",
			Content: map[string]map[string]oas.Schema{
				contentTypes[responseTag]: oas.Generate(reflect.New(out), responseTag),
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
			item["schema"] = oas.Generate(reflect.New(field.Type), "header")["schema"]
		}

		if v, ok := field.Tag.Lookup("form"); ok {
			// 非get 方法 form 会解析为表单
			if route.method == http.MethodGet {
				item["name"] = v
				item["in"] = "query"
				item["schema"] = oas.Generate(reflect.New(field.Type), "form")["schema"]
			} else {
				bodyType["form"] = struct{}{}
			}
		}

		if v, ok := field.Tag.Lookup("uri"); ok {
			item["name"] = v
			item["in"] = "path"
			item["required"] = true
			item["schema"] = oas.Generate(reflect.New(field.Type), "uri")["schema"]
		}

		if item["in"] != nil {
			list = append(list, item)
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
