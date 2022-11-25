package amqp

import (
	"context"

	"github.com/rabbitmq/amqp091-go"
)

type client struct {
	dsn     string
	cancel  context.CancelFunc
	baseCtx context.Context
}

func newClient(ctx context.Context, dsn string) *client {
	subctx, cancel := context.WithCancel(ctx)
	return &client{
		dsn:     dsn,
		baseCtx: subctx,
		cancel:  cancel,
	}
}

type Session struct {
	ch   *amqp091.Channel
	conn *amqp091.Connection
}

// MessageHandle ding
type MessageHandle func(ctx context.Context, msg amqp091.Delivery)

func (c *client) getSession() (*Session, error) {
	conn, err := amqp091.Dial(c.dsn)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &Session{ch, conn}, err
}

type loopHandle func(context.Context, *Session, error) bool

func (c *client) runloop(f loopHandle) error {
	for {
		sess, err := c.getSession()
		if next := f(c.baseCtx, sess, err); !next {
			return nil
		}
		if err == nil {
			sess.conn.Close()
		}
	}
}

func exchangeDeclare(ch *amqp091.Channel, es ...ExchangeOption) error {
	// 自动声明exchange
	for _, x := range es {
		if err := ch.ExchangeDeclare(
			x.Name, x.Kind, x.Durable, x.AutoDelete, false, false, nil,
		); err != nil {
			return err
		}
	}
	return nil
}

func queueDeclare(ch *amqp091.Channel, qs ...QueueOption) error {
	for _, x := range qs {
		if _, err := ch.QueueDeclare(
			x.Queue, x.Durable, x.AutoDelete, false, false, nil,
		); err != nil {
			return err
		}
		for _, v := range x.BindExchange {
			if err := ch.QueueBind(
				x.Queue, v.Exchange, v.RoutingKey, false, nil,
			); err != nil {
				return err
			}
		}
	}
	return nil
}
