package igo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	tr "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TraceExporter 创建trace导出对象的方法类型
type TraceExporter func(ctx context.Context) (trace.SpanExporter, error)

// ExportHTTP 使用HTTP方式 导出上报数据
func ExportHTTP(endpoint string, usehttps bool) TraceExporter {
	return func(ctx context.Context) (trace.SpanExporter, error) {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithTimeout(time.Second * 10),
		}
		if !usehttps {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)
	}
}

// ExportGRPC 使用GRPC方式导出上报数据
func ExportGRPC(endpoint string) TraceExporter {
	return func(ctx context.Context) (trace.SpanExporter, error) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*30)
		defer cancel()
		conn, err := grpc.DialContext(ctx,
			endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			// grpc.WithTimeout(time.Second*10),
		)
		if err != nil {
			return nil, err
		}
		return otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	}
}

// ExportStdout 输出到控制台 仅限测试用 勿使用再生产环境
func ExportStdout(pretty bool) TraceExporter {
	return func(ctx context.Context) (trace.SpanExporter, error) {
		if pretty {
			return stdouttrace.New(stdouttrace.WithPrettyPrint())
		}
		return stdouttrace.New()
	}
}

// ExportEmpty 空实现 启用trace但不导出任何上报数据
func ExportEmpty() func(ctx context.Context) (trace.SpanExporter, error) {
	return func(ctx context.Context) (trace.SpanExporter, error) {
		return &defaultExport{}, nil
	}
}

type defaultExport struct{}

func (d *defaultExport) Shutdown(ctx context.Context) error {
	return nil
}

func (d *defaultExport) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	return nil
}

func newTraceProvider(serviceName string, traceExporter TraceExporter) (*trace.TracerProvider, error) {
	if traceExporter == nil {
		return nil, errors.New("failed to create trace exporter: provider is nil")
	}
	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource:%w", err)
	}

	exp, err := traceExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter:%w", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithResource(res),
		trace.WithSpanProcessor(trace.NewBatchSpanProcessor(exp)),
	)

	return tracerProvider, nil

}

const defaultTracekName = "github.com/parkingwang/igo/igo/core"

// TracerStart 快速的开启一次trace记录
func TracerStart(ctx context.Context, name string) (context.Context, tr.Span) {
	return otel.GetTracerProvider().Tracer(defaultTracekName).Start(ctx, name)
}
