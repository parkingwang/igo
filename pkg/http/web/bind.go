package web

import (
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var autoBindTags = []string{"header", "json", "form", "uri", "query"}

func deepfindTags(t reflect.Type, m map[string]bool) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Anonymous {
				deepfindTags(field.Type, m)
				continue
			}
			for _, v := range autoBindTags {
				if _, ok := field.Tag.Lookup(v); ok {
					m[v] = true
				}
			}
		}
	case reflect.Slice:
		// deepfindTags(t.Elem(), m)
		// m["json"] = true
	}
}

func checkReqParam(ctx *gin.Context, obj any, tags map[string]bool) error {
	if tags["header"] {
		if err := ctx.ShouldBindHeader(obj); err != nil {
			return err
		}
	}
	// 这里需要特殊处理 因为 query使用form的字段 并且只能在GET的时候用 这里改为query
	if tags["query"] {
		// if err := ctx.ShouldBindQuery(obj); err != nil {
		// 	return err
		// }
		if err := ctx.ShouldBindWith(obj, customQuerybind); err != nil {
			return err
		}
	}
	if ctx.ContentType() == "" && tags["json"] {
		if err := ctx.ShouldBindJSON(obj); err != nil {
			return err
		}
	} else {
		if err := ctx.ShouldBind(obj); err != nil {
			return err
		}
	}

	// uri 优先级最高 放到最后防止被覆盖
	if len(ctx.Params) > 0 {
		if err := ctx.ShouldBindUri(obj); err != nil {
			return err
		}
	}
	return nil
}

var customQuerybind = queryBinding{}

type queryBinding struct{}

func (queryBinding) Name() string {
	return "query"
}

func (queryBinding) Bind(req *http.Request, obj any) error {
	values := req.URL.Query()
	return binding.MapFormWithTag(obj, values, "query")
}
