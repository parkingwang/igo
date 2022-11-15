package amqp

import "github.com/rabbitmq/amqp091-go"

type Option func(*option)

func WithDsn(s string) Option {
	return func(o *option) {
		o.dsn = s
	}
}

func WithExchangeDeclare(es ...ExchangeOption) Option {
	return func(o *option) {
		o.exchanges = append(o.exchanges, es...)
	}
}

func WithQueueDeclare(qs ...QueueOption) Option {
	return func(o *option) {
		o.queues = append(o.queues, qs...)
	}
}

// WithOnSubMessage only sub
func WithOnSubMessage(queue string, f MessageHandle) Option {
	return func(o *option) {
		o.messageHandle[queue] = f
	}
}

func WithQos(prefetchCount, prefetchSize int, global bool) Option {
	return func(o *option) {
		o.qos = &qos{prefetchCount, prefetchSize, global}
	}
}

func WithOnError(f func(error)) Option {
	return func(o *option) {
		o.err = f
	}
}

type option struct {
	dsn string
	// 需要声明的交换机
	exchanges     []ExchangeOption
	queues        []QueueOption
	messageHandle map[string]MessageHandle
	err           func(error)
	qos           *qos
}

func defaultOption() *option {
	return &option{
		exchanges:     make([]ExchangeOption, 0),
		queues:        make([]QueueOption, 0),
		messageHandle: make(map[string]MessageHandle),
		err:           func(err error) {},
	}
}

func (o *option) apply(ch *amqp091.Channel) error {
	// 自动声明exchange
	if err := exchangeDeclare(ch, o.exchanges...); err != nil {
		return err
	}
	if err := queueDeclare(ch, o.queues...); err != nil {
		return err
	}
	if o.qos != nil {
		if err := ch.Qos(
			o.qos.prefetchCount,
			o.qos.prefetchSize,
			o.qos.global,
		); err != nil {
			return err
		}
	}
	return nil
}

// ExchangeOption 交换机信息
// sub & pub 都可以用
type ExchangeOption struct {
	// 交换机名称
	Name string
	// 交换机类型 如 fanout direct topic
	Kind string
	// Durable 持久化
	Durable bool
	// AutoDelete设置为 true 表示自动删除 慎用 不是自动删除交换机。
	AutoDelete bool
}

// SubQueueOption sub 声明队列配置
type QueueOption struct {
	Queue      string
	Qos        int
	Durable    bool
	AutoDelete bool
	// 如果配置则自动进行交换机绑定
	BindExchange []QueueBind
}

type QueueBind struct {
	Exchange   string
	RoutingKey string
}

type qos struct {
	prefetchCount int
	prefetchSize  int
	global        bool
}
