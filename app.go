package igo

import (
	"context"
	"os"
	"strings"

	"github.com/parkingwang/igo/internal/trace"
	"github.com/parkingwang/igo/pkg/http/web"
	"github.com/parkingwang/igo/pkg/http/web/oas"
	"github.com/parkingwang/igo/pkg/store/database"
	"github.com/parkingwang/igo/pkg/store/redis"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"golang.org/x/exp/slog"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
)

type Application struct {
	fxProvides    []any
	fxInvokeFuncs []any
	info          AppInfo
}

func New(info AppInfo) *Application {
	cfg := Conf().Child("app")
	slog.SetDefault(slog.New(NewTraceSlogHandler(
		os.Stderr,
		cfg.GetBool("log.addSource"),
		func() slog.Leveler {
			if cfg.GetBool("log.debug") {
				return slog.DebugLevel
			}
			return slog.InfoLevel
		}(),
	)))

	if info.Version == "" {
		info.Version = getVCSVersion()
	}

	slog.Info("init app",
		slog.String("name", info.Name),
		slog.String("version", info.Version),
		slog.String("traceExportType", cfg.GetString("traceExport.type")),
	)

	// enable trace
	tp, err := trace.NewTraceProvider(
		info.Name, info.Version, initTraceExport(),
	)

	if err != nil {
		slog.Error("init tracer povider failed", err)
		os.Exit(1)
	}
	otel.SetTextMapPropagator(b3.New())
	otel.SetTracerProvider(tp)

	// 自动加载pkg/store
	if err := initPkgStore(); err != nil {
		slog.Error("init pkg/store failed", err)
		os.Exit(1)
	}

	return &Application{info: info}
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

func (app *Application) CreateWebServer() *web.Server {
	cfg := Conf().Child("server.web")
	if cfg == nil {
		return web.New()
	}
	var docinfo *oas.DocInfo
	if cfg.GetBool("openapi") {
		docinfo = &oas.DocInfo{
			Title:          app.info.Name,
			Description:    app.info.Description,
			Version:        app.info.Version,
			TermsOfService: "https://github.com/parkingwang/igo",
		}
	}
	return web.New(
		web.WithAddr(cfg.GetString("addr")),
		web.WithDumpRequestBody(cfg.GetBool("dumpRequest")),
		web.WithOpenAPI(docinfo),
	)
}

func initPkgStore() error {
	storecfg := Conf().Child("store")
	if storecfg.IsSet("database") {
		cfg := make(map[string]database.Config)
		if err := storecfg.Decode("database", &cfg); err != nil {
			return err
		}
		return database.RegisterFromConfig(cfg)
	}

	if storecfg.IsSet("redis") {
		cfg := make(map[string]redis.Config)
		if err := storecfg.Decode("redis", &cfg); err != nil {
			return err
		}
		return redis.RegisterFromConfig(cfg)
	}
	return nil
}

func initTraceExport() trace.TraceExporter {
	cfg := Conf().Child("app.traceExport")
	if cfg != nil {
		switch cfg.GetString("type") {
		case "http":
			return trace.ExportHTTP(cfg.GetString("endpoint"), cfg.GetBool("usehttps"))
		case "grpc":
			return trace.ExportGRPC(cfg.GetString("endpoint"))
		case "stdout":
			return trace.ExportStdout(cfg.GetBool("pretty"))
		}
	}

	return trace.ExportEmpty()
}
