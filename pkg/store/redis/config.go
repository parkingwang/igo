package redis

import (
	"time"
)

type Config struct {
	Url          string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	DialTimeout  time.Duration
	MaxRetries   int
}

func RegisterFromConfig(cfgs map[string]Config) error {
	for name, opt := range cfgs {
		if err := RegisterByName(
			name,
			opt.Url,
			WithReadTimeout(opt.ReadTimeout),
			WithWriteTimeout(opt.WriteTimeout),
			WithMaxRetries(opt.MaxRetries),
			WithDialTimeout(opt.DialTimeout),
		); err != nil {
			return err
		}
	}
	return nil
}
