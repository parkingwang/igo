package amqp

import (
	"context"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type client struct {
	dsn string
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

func (c *client) runloop(ctx context.Context, dur time.Duration, f func(context.Context, *Session, error)) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		sess, err := c.getSession()
		f(ctx, sess, err)
		if err == nil {
			sess.conn.Close()
		}
		time.Sleep(dur)
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
