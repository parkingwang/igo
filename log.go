package igo

import (
	"io"
	"os"

	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

func init() {
	slog.SetDefault(slog.New(NewTraceSlogHandler(os.Stderr, false, slog.DebugLevel)))
}

var LogPrivacyAttrKey = []string{
	"password",
	"token",
	"secretkey",
	"accesskey",
}

// NewTraceSlogHandler 集成trace的slog handler
func NewTraceSlogHandler(w io.Writer, addSource bool, lvl slog.Leveler) slog.Handler {
	sets := make(map[string]struct{})
	for _, v := range LogPrivacyAttrKey {
		sets[v] = struct{}{}
	}
	opt := slog.HandlerOptions{
		Level:     lvl,
		AddSource: addSource,
		ReplaceAttr: func(a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(
					slog.TimeKey,
					a.Value.Time().Format("2006-01-02 15:04:05"),
				)
			}
			if _, ok := sets[a.Key]; ok {
				if a.Value.Kind() == slog.StringKind {
					return slog.String(a.Key, replaceLogPrivacyAttrKey(a.Value.String()))
				}
			}
			return a
		},
	}
	return &logTraceHandle{
		opt.NewTextHandler(w),
	}
}

func replaceLogPrivacyAttrKey(s string) string {
	p := []rune(s)
	n := len(p)
	if n == 0 {
		return s
	}
	if n < 3 {
		return string(p[0]) + "*"
	}
	start := n / 3
	for i := start; i < n-start; i++ {
		p[i] = '*'
	}
	return string(p)
}

type logTraceHandle struct {
	*slog.TextHandler
}

func (h *logTraceHandle) Handle(r slog.Record) error {
	span := trace.SpanContextFromContext(r.Context)
	if span.IsValid() {
		r.AddAttrs(slog.String("traceid", span.TraceID().String()))
	}
	return h.TextHandler.Handle(r)
}
