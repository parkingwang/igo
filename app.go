package igo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/parkingwang/igo/core"
	"github.com/parkingwang/igo/pkg/store/database"
	"github.com/parkingwang/igo/pkg/store/redis"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
)

type Application struct {
	name string
	opt  *option
	srvs []core.StartServerfunc
}

type option struct {
	exporter         TraceExporter
	shutdownWaitTime time.Duration
	autoRegiserStore bool
}

type Option func(*option) error

func New(name string, opts ...Option) (*Application, error) {
	if defaultConfig == nil {
		return nil, fmt.Errorf("not found config")
	}
	opt := &option{
		shutdownWaitTime: time.Second * 5,
	}
	for _, apply := range opts {
		if err := apply(opt); err != nil {
			return nil, err
		}
	}
	if opt.exporter != nil {
		// enable trace
		tp, err := newTraceProvider(name, opt.exporter)
		if err != nil {
			return nil, err
		}
		otel.SetTextMapPropagator(b3.New())
		otel.SetTracerProvider(tp)
	}

	if opt.autoRegiserStore {
		if err := autoRegisterStore(); err != nil {
			return nil, fmt.Errorf("autoRegisterStore %w", err)
		}
	}

	app := &Application{
		name: name,
		opt:  opt,
	}

	return app, nil
}

func autoRegisterStore() error {

	if Conf().IsSet("store.database") {
		cfg := make(map[string]database.Config)
		if err := Conf().Decode("store.database", &cfg); err != nil {
			return err
		}
		return database.RegisterFromConfig(cfg)
	}

	if Conf().IsSet("store.redis") {
		cfg := make(map[string]redis.Config)
		if err := Conf().Decode("store.redis", &cfg); err != nil {
			return err
		}
		return redis.RegisterFromConfig(cfg)
	}

	return nil

}

// RegisterServer 注册服务
// 服务需要使用 core.StartServerfunc 函数类型包裹 以便集成优雅停止与重启
func (app *Application) RegisterServer(s ...core.StartServerfunc) {
	app.srvs = append(app.srvs, s...)
}

// Run 开始运行
func (app *Application) Run() error {
	if len(app.srvs) == 0 {
		return errors.New("not found service")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errChan := make(chan error, 1)
	defer close(errChan)

	for _, v := range app.srvs {
		go func(srv core.StartServerfunc) {
			if err := srv(ctx); err != nil {
				errChan <- err
			}
		}(v)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		cancel()
		time.Sleep(app.opt.shutdownWaitTime)
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

// WithOtelTraceExport 链路追踪数据上报配置
func WithOtelTraceExport(tp TraceExporter) Option {
	return func(opt *option) error {
		opt.exporter = tp
		return nil
	}
}

// WithShutdownWaitTime 优雅停止/重启收到信号等待的时间 默认5s
func WithShutdownWaitTime(n time.Duration) Option {
	return func(opt *option) error {
		opt.shutdownWaitTime = n
		return nil
	}
}

// WithAutoRegistorStore 从配置文件自动注册store对象 包括database，redis
// 如果未发现配置 则跳过
func WithAutoRegistorStore() Option {
	return func(opt *option) error {
		opt.autoRegiserStore = true
		return nil
	}
}
