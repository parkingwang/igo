package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/parkingwang/igo/core"
	"github.com/parkingwang/igo/pkg/api/code"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func Server(f func(*gin.Engine, Handler), opts ...Option) core.StartServerfunc {
	opt := &option{
		addr:   ":8080",
		render: DefaultRender,
	}
	for _, o := range opts {
		o(opt)
	}
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.NoRoute(func(ctx *gin.Context) {
		opt.render(ctx, nil,
			code.NewCodeError(
				http.StatusNotFound,
				"route not found",
			),
		)
	})
	e.Use(
		gin.CustomRecovery(func(c *gin.Context, err any) {
			fmt.Println("panic", err)
			opt.render(c, nil,
				code.NewCodeError(
					http.StatusInternalServerError,
					http.StatusText(http.StatusInternalServerError),
				),
			)
		}),
		otelgin.Middleware("apiservice"),
	)
	f(e, handleWarpf(opt.render))
	return func(ctx context.Context) error {
		srv := &http.Server{
			Addr:    opt.addr,
			Handler: e,
		}
		go func() {
			<-ctx.Done()
			sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(sctx); err != nil {
				// 停止失败 强制结束 避免出现kill不掉
				os.Exit(1)
			}
		}()
		return srv.ListenAndServe()
	}
}

func GinCtx(ctx context.Context) *gin.Context {
	c, ok := ctx.(*gin.Context)
	if ok {
		return c
	}
	panic("context can't converto *gin.Context")
}

type option struct {
	render          Renderer
	dumpRequestBody bool
	addr            string
}

type Option func(*option)

// WithResponseRender 自定义响应输出
func WithResponseRender(r Renderer) Option {
	return func(opt *option) {
		opt.render = r
	}
}

// WithDumpRequestBody 是否输出请求体
func WithDumpRequestBody(o bool) Option {
	return func(opt *option) {
		opt.dumpRequestBody = o
	}
}

func WithAddr(addr string) Option {
	return func(o *option) {
		o.addr = addr
	}
}

var errHandleType = errors.New("rpc handle must func(ctx context.Context, in *struct)(out *struct,err error) type")

func checkHandleValid(tp reflect.Type) bool {
	if tp.Kind() != reflect.Func {
		return false
	}

	// check request
	if !(tp.NumIn() == 2 &&
		rtypeContext.Implements(tp.In(0)) &&
		tp.In(1).Kind() == reflect.Ptr &&
		tp.In(1).Elem().Kind() == reflect.Struct) {
		return false
	}

	// check response
	return tp.NumOut() == 2 && rtypeError.Implements(tp.Out(1))
}

func handleWarpf(r Renderer) Handler {
	return func(iface any) gin.HandlerFunc {
		tp := reflect.TypeOf(iface)
		if !checkHandleValid(tp) {
			panic(errHandleType)
		}
		method := reflect.ValueOf(iface)
		reqParamsType := tp.In(1).Elem()

		return func(ctx *gin.Context) {

			q := reflect.New(reqParamsType)
			if reqParamsType != rtypEempty {
				bindq := q.Interface()
				if err := ctx.ShouldBind(bindq); err != nil {
					r(ctx, nil, code.FromError(http.StatusBadRequest, err))
					return
				}
				if len(ctx.Params) > 0 {
					if err := ctx.ShouldBindUri(bindq); err != nil {
						r(ctx, nil, code.FromError(http.StatusBadRequest, err))
						return
					}
				}
			}
			ret := method.Call([]reflect.Value{reflect.ValueOf(ctx), q})
			res := ret[0].Interface()
			err := ret[1].Interface()
			if err == nil {
				switch t := res.(type) {
				case Empty:
				case *Empty:
				default:
					r(ctx, t, nil)
				}
				return
			}
			r(ctx, res, err.(error))
		}
	}
}
