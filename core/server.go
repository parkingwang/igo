package core

import (
	"context"

	"github.com/parkingwang/igo/core/config"
)

// StartServerfunc 启动服务
// 如果上层退出或取消 则内部需要平滑退出
type StartServerfunc func(context.Context) error

var defaultConfig config.Provider

func SetConfig(path string) {
	p, err := config.LoadConfig(path)
	if err != nil {
		panic(err)
	}
	defaultConfig = p
}

func Conf() config.Provider {
	if defaultConfig == nil {
		panic("default config nil")
	}
	return defaultConfig
}
