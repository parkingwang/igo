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
	"github.com/go-playground/validator/v10"
	"github.com/parkingwang/igo/pkg/http/code"
	"github.com/parkingwang/igo/pkg/http/web/oas"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	s.opt.routes.echo()
	{
		if s.opt.docInfo != nil {
			docspec, err := s.opt.routes.ToDoc(*s.opt.docInfo)
			if err != nil {
				slog.Error("build openapi3.0 failed", err)
			}
			e := s.GinEngine()
			e.GET("/debug/doc", func(ctx *gin.Context) {
				ctx.Header("Content-Type", "text/html")
				ctx.Writer.Write(swaggerUIData)
			})
			e.GET("/debug/doc/swagger.json", func(ctx *gin.Context) {
				docspec.Servers = []oas.Server{
					{Url: "http://" + ctx.Request.Host},
				}
				ctx.IndentedJSON(http.StatusOK, docspec)
			})
		}
	}

	l, err := net.Listen("tcp", s.opt.addr)
	if err != nil {
		return err
	}
	// 关闭gin默认的校验
	// 等待所有都读取完成后统一校验
	binding.Validator = nil
	slog.FromContext(ctx).Info("Starting HTTP server", slog.String("addr", s.opt.addr))
	go s.httpsrv.Serve(l)
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.FromContext(ctx).Info("Shutdown HTTP server", slog.String("addr", s.opt.addr))
	return s.httpsrv.Shutdown(ctx)
}

// Router rpc风格的路由
func (s *Server) Router() Router {
	return &route{
		opt: s.opt,
		r:   s.e,
	}
}

// GinEngine 返回原始的ginEngine
func (s *Server) GinEngine() *gin.Engine {
	return s.e
}

// GinContext 返回原始的ginContext
func GinContext(ctx context.Context) (*gin.Context, bool) {
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
		return 2, rtypeError.Implements(tp.Out(1)) &&
			tp.Out(0).Kind() == reflect.Ptr &&
			tp.Out(0).Elem().Kind() == reflect.Struct
	default:
		return n, false
	}
}

var autoBindTags = []string{"header", "json", "form", "uri"}

func handleWarpf(opt *option) Handler {
	return func(iface any) gin.HandlerFunc {
		tp := reflect.TypeOf(iface)
		numOut, ok := checkHandleValid(tp)
		if !ok {
			panic(errHandleType)
		}
		method := reflect.ValueOf(iface)
		reqParamsType := tp.In(1).Elem()

		tags := make(map[string]bool)

		for i := 0; i < reqParamsType.NumField(); i++ {
			tag := reqParamsType.Field(i).Tag
			for _, v := range autoBindTags {
				if _, ok := tag.Lookup(v); ok {
					tags[v] = true
				}
			}
		}

		valider := validator.New()
		valider.SetTagName("binding") // 兼容gin

		return func(ctx *gin.Context) {
			q := reflect.New(reqParamsType)
			if reqParamsType != rtypEempty {
				err := checkReqParam(ctx, q.Interface(), tags)
				if opt.dumpRequestBody {
					// 输出请求体
					slog.Ctx(ctx).LogAttrs(
						slog.InfoLevel,
						"gin.dumpRequest",
						slog.String("data", fmt.Sprintf("%+v", q.Elem())),
					)
				}
				if err == nil {
					err = valider.Struct(q.Interface())
				}
				if err != nil {
					warpRender(opt, ctx, nil, code.NewBadRequestError(err))
					return
				}
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

func checkReqParam(ctx *gin.Context, obj any, tags map[string]bool) error {
	if tags["header"] {
		if err := ctx.ShouldBindHeader(obj); err != nil {
			return err
		}
	}
	// 这里需要特殊处理 因为 query使用form的字段 并且只能在GET的时候用
	// 当请求时json时 又由query绑定的tag:form将会失效
	if (ctx.Request.Method != http.MethodGet) && (ctx.ContentType() == binding.MIMEJSON) {
		if tags["form"] {
			if err := ctx.ShouldBindQuery(obj); err != nil {
				return err
			}
		}
	}
	if err := ctx.ShouldBind(obj); err != nil {
		return err
	}
	// uri 优先级最高 放到最后防止被覆盖
	if tags["uri"] {
		if len(ctx.Params) > 0 {
			if err := ctx.ShouldBindUri(obj); err != nil {
				return err
			}
		}
	}

	return nil
}

func middleware(service string, opts ...Option) gin.HandlerFunc {
	tracer := otel.GetTracerProvider().Tracer("github.com/parkingwang/igo/pkg/http/web")
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
			span.SetStatus(codes.Error, c.Errors.String())
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
