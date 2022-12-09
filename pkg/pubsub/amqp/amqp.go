package amqp

import (
	"context"
	"net"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

func createConnection(ctx context.Context, name, dsn string) (*amqp091.Connection, error) {
	config := amqp091.Config{
		Properties: amqp091.NewConnectionProperties(),
	}
	config.Properties.SetClientConnectionName(name)
	config.Heartbeat = time.Second * 10
	config.Dial = func(network, addr string) (net.Conn, error) {
		d := net.Dialer{}
		return d.DialContext(ctx, network, addr)
	}
	return amqp091.DialConfig(dsn, config)
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
				x.Queue, v.RoutingKey, v.Exchange, false, nil,
			); err != nil {
				return err
			}
		}
	}
	return nil
}
