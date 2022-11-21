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
	f(g.e, handleWarpf(g.opt.render))
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
					r(ctx, nil, code.NewCodeError(http.StatusBadRequest, err.Error()))
					return
				}
				if len(ctx.Params) > 0 {
					if err := ctx.ShouldBindUri(bindq); err != nil {
						r(ctx, nil, code.NewCodeError(http.StatusBadRequest, err.Error()))
						return
					}
				}
			}
			ret := method.Call([]reflect.Value{reflect.ValueOf(ctx), q})
			if err := ret[1].Interface(); err != nil {
				r(ctx, nil, err.(error))
				return
			}
			switch t := ret[0].Interface().(type) {
			case Empty:
			case *Empty:
			default:
				r(ctx, t, nil)
			}
		}
	}
}

func middleware(service string, opts ...Option) gin.HandlerFunc {
	tracer := otel.GetTracerProvider().Tracer("github.com/parkingwang/igo/pkg/ginserver")
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

		logger := slog.With(slog.String("traceid", span.SpanContext().TraceID().String()))
		c.Request = c.Request.WithContext(slog.NewContext(ctx, logger))

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
		logger.LogAttrs(loglvl, "gin.access", logattrs...)
	}
}
