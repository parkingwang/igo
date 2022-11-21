package amqp

import (
	"context"
	"errors"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Pub struct {
	msg chan *pubChanData
	opt *option
}

func NewPub(ctx context.Context, opts ...Option) *Pub {
	opt := defaultOption()
	for _, v := range opts {
		v(opt)
	}
	p := &Pub{msg: make(chan *pubChanData, 1024)}
	go (&client{dsn: opt.dsn}).runloop(
		ctx,
		time.Second*10,
		p.loopHandle,
	)
	return p
}

func (p *Pub) loopHandle(ctx context.Context, s *Session, serr error) {
	if serr != nil {
		p.opt.err(serr)
		return
	}
	if err := p.opt.apply(s.ch); err != nil {
		p.opt.err(err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			// wait 消息发送完
			// 消息消费完再退出
			close(p.msg)
		case msg, ok := <-p.msg:
			if !ok {
				return
			}
			err := s.ch.PublishWithContext(
				msg.ctx, msg.exchange, msg.key, false, false, msg.data,
			)
			msg.errchan <- err
			if err != nil {
				if errors.Is(err, amqp091.ErrClosed) {
					return
				}
			}
		}
	}
}

type pubChanData struct {
	ctx      context.Context
	exchange string
	key      string
	data     amqp091.Publishing
	errchan  chan error
}

func (p *Pub) PublishMsg(ctx context.Context, exchange, key string, msg amqp091.Publishing) error {
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

	errchan := make(chan error, 1)
	defer close(errchan)

	m := &pubChanData{
		ctx:      ctx,
		exchange: exchange,
		key:      key,
		data:     msg,
		errchan:  errchan,
	}
	p.msg <- m
	select {
	case <-ctx.Done():
		err := ctx.Err()
		if err != nil {
			span.RecordError(err)
		}
		return err
	case err := <-errchan:
		if err != nil {
			span.RecordError(err)
		}
		return err
	}
}

func (p *Pub) Publish(ctx context.Context, exchange, key string, data []byte) error {
	msg := amqp091.Publishing{
		Body:         data,
		DeliveryMode: amqp091.Transient,
		Priority:     0,
	}
	return p.PublishMsg(ctx, exchange, key, msg)
}
