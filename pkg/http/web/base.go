package web

import (
	"context"
	"reflect"

	"github.com/gin-gonic/gin"
)

// Handler ginhandler包裹器 负责将rpc模式转为gin handler
type Handler func(any) gin.HandlerFunc

// Empty 空参数定义 此类型不参与参数解析
type Empty struct{}

var (
	rtypEempty   = reflect.TypeOf(&Empty{})
	rtypeContext = reflect.TypeOf(context.Background())
	rtypeError   = reflect.TypeOf(errHandleType)
)
