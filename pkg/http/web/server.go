package web

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/parkingwang/igo/pkg/http/code"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

type Server struct {
	opt     *option
	e       *gin.Engine
	httpsrv *http.Server
}

func (g *Server) Route(f func(*gin.Engine, Handler)) {
	f(g.e, handleWarpf(g.opt))
}

func New(opts ...Option) *Server {
	opt := defaultOption()
	for _, o := range opts {
		o(opt)
	}
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.ContextWithFallback = true
	e.NoRoute(func(ctx *gin.Context) {
		opt.render(ctx, nil, code.NewNotfoundError("route not found"))
	})
	e.Use(
		middleware("apiservice"),
		gin.CustomRecovery(func(c *gin.Context, err any) {
			slog.Ctx(c).LogAttrs(slog.ErrorLevel, "gin.panic", slog.Any("err", err))
			c.Abort()
			opt.render(c, nil,
				code.NewCodeError(
					http.StatusInternalServerError,
					http.StatusText(http.StatusInternalServerError),
				),
			)
		}),
	)

	pprof.Register(e)

	return &Server{
		opt: opt,
		e:   e,
		httpsrv: &http.Server{
			Handler: e,
		},
	}
}

func (s *Server) Start(ctx context.Context) error {
	for _, v := range s.opt.routesInfo {
		if v.isDir {
			if len(v.children) == 0 {
				continue
			}
			fmt.Println("[api] ", v.path, v.comment)
			for _, h := range v.children {
				fmt.Println("[api] ", h.path, h.method, h.pcName, h.comment)
			}
			fmt.Println("[api] ")
		} else {
			fmt.Println("[api] ", v.method, v.path, v.pcName, v.comment)
		}
	}

	l, err := net.Listen("tcp", s.opt.addr)
	if err != nil {
		return err
	}
	slog.FromContext(ctx).Info("Starting HTTP server", slog.String("addr", s.opt.addr))
	go s.httpsrv.Serve(l)
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.FromContext(ctx).Info("Shutdown HTTP server", slog.String("addr", s.opt.addr))
	return s.httpsrv.Shutdown(ctx)
}

// RPCRouter rpc风格的路由
func (s *Server) RPCRouter() Router {
	return &route{
		opt: s.opt,
		r:   s.e,
	}
}

// RawRouter 返回原始的ginEngine
func (s *Server) RawRouter() *gin.Engine {
	return s.e
}

// RawContext 返回原始的ginContext
func RawContext(ctx context.Context) (*gin.Context, bool) {
	c, ok := ctx.(*gin.Context)
	return c, ok
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

func shouldBind(ctx *gin.Context, obj any) error {
	contentType := ctx.ContentType()
	method := ctx.Request.Method
	// 如果未没有指定contentType则按json
	// 否则bind会失效
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		if contentType == "" {
			contentType = binding.MIMEJSON
		}
	}
	b := binding.Default(method, contentType)
	return ctx.ShouldBindWith(obj, b)
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
				if len(ctx.Params) > 0 {
					if err := ctx.ShouldBindUri(bindq); err != nil {
						warpRender(opt, ctx, nil, code.NewBadRequestError(err))
						return
					}
				}
				if err := shouldBind(ctx, bindq); err != nil {
					warpRender(opt, ctx, nil, code.NewBadRequestError(err))
					return
				}
			}
			if opt.dumpRequestBody {
				// 输出请求体
				slog.Ctx(ctx).LogAttrs(
					slog.InfoLevel,
					"gin.dumpRequest",
					slog.String("data", fmt.Sprintf("%+v", q.Elem())),
				)
			}
			ret := method.Call([]reflect.Value{reflect.ValueOf(ctx), q})
			if e := ret[numOut-1].Interface(); e != nil {
				warpRender(opt, ctx, nil, e.(error))
				return
			}
			if numOut == 2 {
				warpRender(opt, ctx, ret[0].Interface(), nil)
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

		log := slog.Ctx(ctx)
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

		if rerr := c.GetString("gin.response.err"); rerr != "" {
			logattrs = append(logattrs,
				slog.String("response.error", rerr),
			)
		}

		log.LogAttrs(loglvl, "gin.access", logattrs...)
	}
}