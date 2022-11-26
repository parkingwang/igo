package config

import (
	"time"

	"github.com/spf13/viper"
)

type Provider interface {
	GetString(key string) string
	GetInt(key string) int
	GetInt64(key string) int64
	GetFloat64(key string) float64
	GetDuration(key string) time.Duration
	GetTime(key string) time.Time
	GetBool(key string) bool
	GetStringMap(key string) map[string]any
	GetStringMapString(key string) map[string]string
	GetStringMapStringSlice(key string) map[string][]string
	GetStringSlice(key string) []string
	GetIntSlice(key string) []int
	Get(key string) any
	Set(key string, value any)
	SetDefault(key string, value any)
	IsSet(key string) bool

	Child(key string) Provider
	Decode(key string, value any) error
}

func LoadConfig(path string) (Provider, error) {
	p := viper.New()
	p.SetConfigFile(path)
	if err := p.ReadInConfig(); err != nil {
		return nil, err
	}
	return &defaultProvider{Viper: p}, nil
}

type defaultProvider struct {
	*viper.Viper
}

func (p *defaultProvider) Child(key string) Provider {
	if p.Viper != nil {
		sub := p.Viper.Sub(key)
		if sub == nil {
			return nil
		}
		return &defaultProvider{Viper: sub}
	}
	return nil
}

func (p *defaultProvider) Decode(key string, value any) error {
	if p.Viper != nil {
		return p.Viper.UnmarshalKey(key, value)
	}
	return nil
}
