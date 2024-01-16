package database

import (
	"time"
)

type Config struct {
	Url             string
	Driver          string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
}

func RegisterFromConfig(cfgs map[string]Config) error {
	for name, opt := range cfgs {
		if err := RegisterByName(
			name,
			opt.Url,
			opt.Driver,
			WithMaxOpenConns(opt.MaxOpenConns),
			WithMaxIdleConns(opt.MaxIdleConns),
			WithConnMaxIdleTime(opt.ConnMaxIdleTime),
		); err != nil {
			return err
		}
	}
	return nil
}
