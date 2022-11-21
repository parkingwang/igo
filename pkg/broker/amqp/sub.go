package amqp

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

type Sub struct {
	handle map[string]MessageHandle
	opt    *option
}

func NewConsumer(opts ...Option) *Sub {
	opt := defaultOption()
	for _, v := range opts {
		v(opt)
	}
	return &Sub{
		opt:    opt,
		handle: make(map[string]MessageHandle),
	}

}

func (s *Sub) Handle(queue string, h MessageHandle) {
	if _, ok := s.handle[queue]; ok {
		slog.Warn("handle alreay used", "queue", queue)
	}
	s.handle[queue] = func(ctx context.Context, msg amqp091.Delivery) {
		ctx, span := s.opt.tracker.Start(ctx, "amqp.consumer",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(attribute.String("queue", queue)),
		)
		logger := slog.With(slog.String("traceid", span.SpanContext().TraceID().String()))
		defer func() {
			if e := recover(); e != nil {
				span.SetStatus(codes.Error, fmt.Sprintf("panic %v", e))
			}
			span.End()
		}()
		h(slog.NewContext(ctx, logger), msg)
	}
}

func (s *Sub) Run(ctx context.Context) error {
	opt := s.opt
	if opt.dsn == "" {
		return fmt.Errorf("dsn error")
	}
	if len(opt.messageHandle) == 0 {
		return fmt.Errorf("messageHandle not found")
	}
	clt := &client{dsn: opt.dsn}
	done := make(chan struct{}, 1)
	safeDone := func() func() {
		var one sync.Once
		return func() {
			one.Do(func() {
				close(done)
			})
		}
	}()
	return clt.runloop(ctx, time.Second*10, func(ctx context.Context, s *Session, serr error) {
		if serr != nil {
			opt.err(serr)
			return
		}
		if err := opt.apply(s.ch); err != nil {
			opt.err(err)
			return
		}
		consumerTag, _ := os.Hostname()
		for qname, handle := range opt.messageHandle {
			msgs, err := s.ch.Consume(qname, consumerTag, false, false, false, false, nil)
			if err != nil {
				opt.err(err)
				return
			}
			for i := 0; i < opt.messageHandleWorker; i++ {
				go func(h MessageHandle) {
					for msg := range msgs {
						h(fromDelivery(msg), msg)
					}
					safeDone()
				}(handle)
			}
		}
		select {
		case <-done:
		case <-ctx.Done():
			safeDone()
		}
	})
}

func fromDelivery(d amqp091.Delivery) context.Context {
	ctx := context.TODO()
	tablemap := make(map[string]string)
	for k, v := range d.Headers {
		if a, ok := v.(string); ok {
			tablemap[k] = a
		}
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(tablemap))
}
