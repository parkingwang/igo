package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/parkingwang/igo/pkg/http/code"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

type Server struct {
	opt *option
	e   *gin.Engine
}

func (g *Server) Route(f func(*gin.Engine, Handler)) {
	f(g.e, handleWarpf(g.opt))
}

func New(opts ...Option) *Server {
	opt := &option{
		addr:   ":8080",
		render: DefaultRender,
	}
	for _, o := range opts {
		o(opt)
	}
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.ContextWithFallback = true
	e.NoRoute(func(ctx *gin.Context) {
		opt.render(ctx, nil,
			code.NewCodeError(
				http.StatusNotFound,
				"route not found",
			),
		)
	})
	e.Use(
		middleware("apiservice"),
		gin.CustomRecovery(func(c *gin.Context, err any) {
			slog.FromContext(c).LogAttrs(slog.ErrorLevel, "gin.panic", slog.Any("err", err))
			c.Abort()
			opt.render(c, nil,
				code.NewCodeError(
					http.StatusInternalServerError,
					http.StatusText(http.StatusInternalServerError),
				),
			)
		}),
	)

	return &Server{
		opt: opt,
		e:   e,
	}
}

func (g *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:    g.opt.addr,
		Handler: g.e,
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

func GinCtx(ctx context.Context) *gin.Context {
	c, ok := ctx.(*gin.Context)
	if ok {
		return c
	}
	return nil
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

func checkHandleValid(tp reflect.Type) (int, bool) {
	if tp.Kind() != reflect.Func {
		return 0, false
	}

	// check request
	if !(tp.NumIn() == 2 &&
		rtypeContext.Implements(tp.In(0)) &&
		tp.In(1).Kind() == reflect.Ptr &&
		tp.In(1).Elem().Kind() == reflect.Struct) {
		return 0, false
	}

	// check response
	// 一个返回值的话 必须是error
	// 两个返回值 最有一个一定是error
	// 其他都不支持
	switch n := tp.NumOut(); n {
	case 1:
		return 1, rtypeError.Implements(tp.Out(0))
	case 2:
		return 2, rtypeError.Implements(tp.Out(1))
	default:
		return n, false
	}
}

func handleWarpf(opt *option) Handler {
	return func(iface any) gin.HandlerFunc {
		tp := reflect.TypeOf(iface)
		numOut, ok := checkHandleValid(tp)
		if !ok {
			panic(errHandleType)
		}
		method := reflect.ValueOf(iface)
		reqParamsType := tp.In(1).Elem()

		return func(ctx *gin.Context) {

			q := reflect.New(reqParamsType)
			if reqParamsType != rtypEempty {
				bindq := q.Interface()
				if err := ctx.ShouldBind(bindq); err != nil {
					opt.render(ctx, nil, code.NewCodeError(http.StatusBadRequest, err.Error()))
					return
				}
				if len(ctx.Params) > 0 {
					if err := ctx.ShouldBindUri(bindq); err != nil {
						opt.render(ctx, nil, code.NewCodeError(http.StatusBadRequest, err.Error()))
						return
					}
				}
			}
			if opt.dumpRequestBody {
				slog.FromContext(ctx).Info("request dump", slog.Any("data", q))
			}
			ret := method.Call([]reflect.Value{reflect.ValueOf(ctx), q})
			if numOut == 1 {
				opt.render(ctx, nil, ret[0].Interface().(error))
			} else {
				opt.render(ctx, ret[0].Interface(), ret[1].Interface().(error))
			}

		}
	}
}

func middleware(service string, opts ...Option) gin.HandlerFunc {
	tracer := otel.GetTracerProvider().Tracer("github.com/parkingwang/igo/pkg/router")
	txtpropagator := otel.GetTextMapPropagator()
	return func(c *gin.Context) {
		savedCtx := c.Request.Context()
		defer func() {
			c.Request = c.Request.WithContext(savedCtx)
		}()

		start := time.Now()
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path = path + "?" + raw
		}

		ctx := txtpropagator.Extract(savedCtx, propagation.HeaderCarrier(c.Request.Header))
		opts := []trace.SpanStartOption{
			trace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", c.Request)...),
			trace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(c.Request)...),
			trace.WithAttributes(semconv.HTTPServerAttributesFromHTTPRequest(service, c.FullPath(), c.Request)...),
			trace.WithSpanKind(trace.SpanKindServer),
		}
		spanName := c.FullPath()
		if spanName == "" {
			spanName = fmt.Sprintf("HTTP %s route not found", c.Request.Method)
		}
		ctx, span := tracer.Start(ctx, spanName, opts...)
		defer span.End()

		log := slog.FromContext(ctx)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		status := c.Writer.Status()
		attrs := semconv.HTTPAttributesFromHTTPStatusCode(status)
		spanStatus, spanMessage := semconv.SpanStatusFromHTTPStatusCodeAndSpanKind(status, trace.SpanKindServer)
		span.SetAttributes(attrs...)
		span.SetStatus(spanStatus, spanMessage)

		loglvl := slog.InfoLevel
		logattrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("ip", c.ClientIP()),
			slog.Int("status", status),
			slog.Int("size", c.Writer.Size()),
			slog.Duration("latency", time.Since(start)),
		}

		if len(c.Errors) > 0 {
			span.SetAttributes(attribute.String("gin.errors", c.Errors.String()))
			loglvl = slog.ErrorLevel
			logattrs = append(logattrs, slog.String("err", c.Errors.ByType(gin.ErrorTypePrivate).String()))
		}
		log.LogAttrs(loglvl, "gin.access", logattrs...)
	}
}
