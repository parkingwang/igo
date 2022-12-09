package amqp

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Producer struct {
	opt *option

	conn *amqp091.Connection
	ch   *amqp091.Channel

	tryCnnection atomic.Bool
	closed       atomic.Bool
}

func NewProducer(ctx context.Context, opts ...Option) (*Producer, error) {
	opt := defaultOption()
	for _, v := range opts {
		v(opt)
	}
	p := &Producer{opt: opt}
	if err := p.createConnChannel(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Producer) createConnChannel(ctx context.Context) error {
	if p.closed.Load() {
		return amqp091.ErrClosed
	}
	if p.tryCnnection.CompareAndSwap(false, true) {
		defer p.tryCnnection.Store(false)
		if p.ch != nil {
			p.ch.Close()
			p.conn.Close()
		}
		conn, err := createConnection(ctx, p.opt.name, p.opt.dsn)
		if err != nil {
			p.opt.err(err)
			time.Sleep(time.Second * 2)
			return err
		}
		ch, err := conn.Channel()
		if err != nil {
			conn.Close()
			p.opt.err(err)
			return err
		}
		p.conn = conn
		p.ch = ch

		if err := p.opt.apply(ch); err != nil {
			p.opt.err(err)
		}
	}
	return nil
}

func (p *Producer) Close() error {
	p.closed.Store(true)
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

func (p *Producer) PublishMsg(ctx context.Context, exchange, key string, msg amqp091.Publishing) error {
	if msg.Headers == nil {
		msg.Headers = make(amqp091.Table)
	}
	ctx, span := p.opt.tracker.Start(ctx, "amqp.publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(attribute.String("exchange", exchange)),
		trace.WithAttributes(attribute.String("routingkey", key)),
	)
	defer span.End()
	tablemap := make(map[string]string)
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(tablemap))
	for k, v := range tablemap {
		msg.Headers[k] = v
	}

	if p.tryCnnection.Load() {
		return amqp091.ErrClosed
	}
	if err := p.ch.PublishWithContext(ctx, exchange, key, false, false, msg); err != nil {
		if p.ch.IsClosed() {
			return p.createConnChannel(ctx)
		} else {
			return err
		}
	}
	return nil

}

func (p *Producer) Publish(ctx context.Context, exchange, key string, data []byte) error {
	msg := amqp091.Publishing{
		Body:         data,
		DeliveryMode: amqp091.Transient,
		Priority:     0,
	}
	return p.PublishMsg(ctx, exchange, key, msg)
}
