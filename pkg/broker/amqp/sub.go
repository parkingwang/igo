package amqp

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Sub struct {
	handle map[string]MessageHandle
	opt    *option
}

func NewSub(opts ...Option) *Sub {
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
		fmt.Println("warn")
	}
	s.handle[queue] = h
}

func (s *Sub) Wait(ctx context.Context) error {
	opt := s.opt
	if opt.dsn == "" {

	}
	if len(opt.messageHandle) == 0 {

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
			go func(h MessageHandle) {
				for msg := range msgs {
					h(fromDelivery(msg), msg)
				}
				safeDone()
			}(handle)
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
