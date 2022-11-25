package amqp

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

type Consumer struct {
	handle map[string]MessageHandle
	opt    *option
	clt    *client
}

func NewConsumer(opts ...Option) *Consumer {
	opt := defaultOption()
	for _, v := range opts {
		v(opt)
	}
	return &Consumer{
		opt:    opt,
		handle: make(map[string]MessageHandle),
	}

}

func (s *Consumer) Subcrite(queue string, h MessageHandle) {
	if _, ok := s.handle[queue]; ok {
		slog.Warn("amqp handle alreay used", "queue", queue)
	}
	s.handle[queue] = func(ctx context.Context, msg amqp091.Delivery) {
		ctx, span := s.opt.tracker.Start(ctx, "amqp.consumer",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(attribute.String("queue", queue)),
		)
		defer func() {
			if e := recover(); e != nil {
				span.SetStatus(codes.Error, fmt.Sprintf("panic %v", e))
			}
			span.End()
		}()
		h(ctx, msg)
	}
}

func (s *Consumer) do(ctx context.Context, sess *Session, serr error) bool {
	if serr != nil {
		s.opt.err(serr)
		return true
	}
	if err := s.opt.apply(sess.ch); err != nil {
		s.opt.err(err)
		return true
	}
	subctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	consumerTag, _ := os.Hostname()
	for qname, handle := range s.opt.messageHandle {
		msgs, err := sess.ch.Consume(qname, consumerTag, false, false, false, false, nil)
		if err != nil {
			s.opt.err(err)
			return true
		}
		for i := 0; i < s.opt.messageHandleWorker; i++ {
			go func(h MessageHandle) {
				for msg := range msgs {
					h(fromDelivery(msg), msg)
				}
				// 通知关闭session
				// 以便其他handler也可以退出
				cancel()
			}(handle)
		}
	}
	select {
	case <-subctx.Done():
		sess.ch.Close()
		return true
	case <-ctx.Done():
		// 等待一会 让执行中的任务执行完
		time.Sleep(time.Second * 2)
		return false
	}
}

func (s *Consumer) Start(ctx context.Context) error {
	opt := s.opt
	if opt.dsn == "" {
		return fmt.Errorf("dsn error")
	}
	if len(opt.messageHandle) == 0 {
		return fmt.Errorf("messageHandle not found")
	}
	s.clt = newClient(ctx, opt.dsn)
	go s.clt.runloop(s.do)
	return nil
}

func (s *Consumer) Stop(ctx context.Context) error {
	s.clt.cancel()
	return nil
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
