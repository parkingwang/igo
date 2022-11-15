package redis

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/go-redis/redis/extra/redisotel/v8"
	"github.com/go-redis/redis/v8"
)

var rs map[string]*redis.Client

const defaultName = "default"

// Register 使用默认名称 default 进行注册
func Register(dsn string, opts ...Option) error {
	return RegisterByName(defaultName, dsn, opts...)
}

// RegisterByName 注册redis
// dns  tcp://aaaaaa@127.0.0.1:5672/0
func RegisterByName(name, dsn string, opts ...Option) error {
	if _, ok := rs[name]; ok {
		return fmt.Errorf("db %s alreay register", name)
	}
	opt, err := parseDsn(dsn)
	if err != nil {
		return fmt.Errorf("redis: parse dsn %w", err)
	}
	for _, o := range opts {
		if err := o(opt); err != nil {
			return err
		}
	}
	c := redis.NewClient(opt)
	c.AddHook(redisotel.NewTracingHook())
	rs[name] = c
	return nil
}

// Get 获取已注册的redis client实例
// 如果未注册则会panic
func Get(name ...string) *redis.Client {
	var n string
	if len(name) == 0 || name[0] == "" {
		n = defaultName
	} else {
		n = name[0]
	}
	c, ok := rs[n]
	if ok {
		return c
	}
	panic(fmt.Sprintf("redis %s not registor", n))
}

// Option redis选项
type Option func(*redis.Options) error

func WithMaxRetries(n int) Option {
	return func(o *redis.Options) error {
		o.MaxRetries = n
		return nil
	}
}

func WithDialTimeout(n time.Duration) Option {
	return func(o *redis.Options) error {
		o.DialTimeout = n
		return nil
	}
}

func WithReadTimeout(n time.Duration) Option {
	return func(o *redis.Options) error {
		o.ReadTimeout = n
		return nil
	}
}

func WithWriteTimeout(n time.Duration) Option {
	return func(o *redis.Options) error {
		o.WriteTimeout = n
		return nil
	}
}

func parseDsn(dsn string) (*redis.Options, error) {
	x, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}
	db, _ := strconv.Atoi(x.Path[1:])
	user := x.User.Username()
	pwd, ok := x.User.Password()
	if !ok {
		if user != "" {
			pwd = user
			user = ""
		}
	}

	opt := &redis.Options{
		Network:  x.Scheme,
		Addr:     x.Host,
		Username: user,
		Password: pwd,
		DB:       db,
	}

	return opt, nil
}
