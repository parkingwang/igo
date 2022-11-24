package log

import (
	"io"

	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

var scrkeyTokens = []string{
	"password",
	"token",
	"secretkey",
	"accesskey",
}

func NewTraceHandler(w io.Writer) slog.Handler {
	sets := make(map[string]struct{})
	for _, v := range scrkeyTokens {
		sets[v] = struct{}{}
	}
	opt := slog.HandlerOptions{
		ReplaceAttr: func(a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(
					slog.TimeKey,
					a.Value.Time().Format("2006-01-02 15:04:05"),
				)
			}
			if _, ok := sets[a.Key]; ok {
				if a.Value.Kind() == slog.StringKind {
					return slog.String(a.Key, replaceToken(a.Value.String()))
				}
			}
			return a
		},
	}
	return &traceHandle{
		opt.NewTextHandler(w),
	}
}

func replaceToken(s string) string {
	n := len(s)
	if n < 3 {
		return s
	}
	start := n / 3
	end := start * 2
	news := []byte(s)
	for i := start; i < end; i++ {
		news[i] = '*'
	}
	return string(news)
}

type traceHandle struct {
	*slog.TextHandler
}

func (h *traceHandle) Handle(r slog.Record) error {
	span := trace.SpanContextFromContext(r.Context)
	r.AddAttrs(slog.String("traceid", span.TraceID().String()))
	return h.TextHandler.Handle(r)
}
