package igo

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"golang.org/x/exp/slog"

	"github.com/parkingwang/igo/pkg/store/database"
	"github.com/parkingwang/igo/pkg/store/redis"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
)

type Application struct {
	name string
	opt  *option

	fxProvides    []any
	fxInvokeFuncs []any
}

type option struct {
	exporter TraceExporter
}

type Option func(*option)

// WithOtelTraceExport 链路追踪数据上报配置
func WithOtelTraceExport(tp TraceExporter) Option {
	return func(opt *option) {
		opt.exporter = tp
	}
}

func New(name string, opts ...Option) *Application {
	opt := &option{
		exporter: ExportEmpty(),
	}
	for _, apply := range opts {
		apply(opt)
	}

	// enable trace
	tp, err := newTraceProvider(name, opt.exporter)
	if err != nil {
		slog.Error("init tracer povider failed", err)
		os.Exit(1)
	}
	otel.SetTextMapPropagator(b3.New())
	otel.SetTracerProvider(tp)

	app := &Application{
		name: name,
		opt:  opt,
	}

	return app
}

// AutoRegisterStore 自动注册数据库和缓存依赖
// 需要加载配置文件 并且存在key store.database/store.redis
func (app *Application) AutoRegisterStore() error {
	config := Conf()
	if config == nil {
		return fmt.Errorf("config not set")
	}
	if config.IsSet("store.database") {
		cfg := make(map[string]database.Config)
		if err := config.Decode("store.database", &cfg); err != nil {
			return err
		}
		return database.RegisterFromConfig(cfg)
	}

	if config.IsSet("store.redis") {
		cfg := make(map[string]redis.Config)
		if err := config.Decode("store.redis", &cfg); err != nil {
			return err
		}
		return redis.RegisterFromConfig(cfg)
	}
	return nil
}

// Provide 依赖注入构造器
func (app *Application) Provide(provide ...any) {
	app.fxProvides = append(app.fxProvides, provide...)
}

// Invoke 注册调用
func (app *Application) Invoke(funcs ...any) {
	app.fxInvokeFuncs = append(app.fxInvokeFuncs, funcs...)
}

func fxLifecycle(srvs []Servicer, lc fx.Lifecycle) {
	for _, v := range srvs {
		lc.Append(fx.Hook{
			OnStart: v.Start,
			OnStop:  v.Stop,
		})
	}
}

func (app *Application) Run(srv ...any) {
	for _, v := range srv {
		app.fxProvides = append(app.fxProvides, asServicer(v))
	}
	fxapp := fx.New(
		fx.WithLogger(func() fxevent.Logger {
			return &fxInjectLogger{
				baselog: slog.With(slog.String("type", "igo")),
			}
		}),
		fx.Provide(app.fxProvides...),
		fx.Invoke(app.fxInvokeFuncs...),
		fx.Invoke(
			fx.Annotate(
				fxLifecycle,
				fx.ParamTags(`group:"services"`),
			),
		),
	)
	fxapp.Run()
}

func asServicer(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(Servicer)),
		fx.ResultTags(`group:"services"`),
	)
}

type fxInjectLogger struct {
	baselog *slog.Logger
}

func (m *fxInjectLogger) LogEvent(event fxevent.Event) {
	switch e := event.(type) {
	case *fxevent.Provided:
		if e.Err != nil {
			m.baselog.Error("provided error encountered while applying options", e.Err)
		}
	case *fxevent.Invoked:
		if e.Err != nil {
			m.baselog.Error("invoked failed", e.Err, slog.String("function", e.FunctionName))
		}
	case *fxevent.Stopping:
		m.baselog.Info("received signal", slog.String("signal", strings.ToUpper(e.Signal.String())))
	case *fxevent.Stopped:
		if e.Err != nil {
			m.baselog.Error("stop failed", e.Err)
		}
	case *fxevent.Started:
		if e.Err != nil {
			m.baselog.Error("start failed", e.Err)
		} else {
			m.baselog.Info("started")
		}
	}
}

// Servicer 服务接口
type Servicer interface {
	Start(context.Context) error
	Stop(context.Context) error
}
