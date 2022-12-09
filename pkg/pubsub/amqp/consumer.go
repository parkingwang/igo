package amqp

import (
	"context"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Consumer struct {
	opt    *option
	cancel context.CancelFunc
}

// MessageHandle ding
type MessageHandle func(ctx context.Context, msg amqp091.Delivery)

func NewConsumer(opts ...Option) *Consumer {
	opt := defaultOption()
	for _, v := range opts {
		v(opt)
	}
	return &Consumer{
		opt: opt,
	}

}

func (s *Consumer) do(ctx context.Context, c *amqp091.Connection, serr error) bool {
	if serr != nil {
		s.opt.err(serr)
		return true
	}
	subctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var applyChannel bool

	chs := make([]*amqp091.Channel, 0)
	defer func() {
		for _, v := range chs {
			v.Close()
		}
	}()
	for qname := range s.opt.messageHandle {
		ch, err := c.Channel()
		if err != nil {
			s.opt.err(err)
			return true
		}
		chs = append(chs, ch)
		if !applyChannel {
			if err := s.opt.apply(ch); err != nil {
				s.opt.err(err)
				return true
			}
			applyChannel = true
		}
		name := qname
		msgs, err := ch.Consume(name, "", false, false, false, false, nil)
		if err != nil {
			s.opt.err(err)
			return true
		}
		go func(h MessageHandle) {
			for msg := range msgs {
				h(fromDelivery(msg), msg)
			}
			// 以便其他handler也可以退出
			cancel()
		}(s.opt.messageHandle[name])
	}
	select {
	case <-subctx.Done():
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

	lifeCtx, cannel := context.WithCancel(context.Background())
	s.cancel = cannel

	conn, err := createConnection(ctx, opt.name, opt.dsn)
	if err != nil {
		return err
	}
	go func() {
		for {
			next := s.do(lifeCtx, conn, err)
			if !next {
				return
			}
			time.Sleep(time.Second * 10)
			c, subcancel := context.WithTimeout(lifeCtx, time.Second*5)
			conn, err = createConnection(c, opt.name, opt.dsn)
			subcancel()
		}
	}()
	return nil
}

func (s *Consumer) Stop(ctx context.Context) error {
	s.cancel()
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
