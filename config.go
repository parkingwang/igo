package igo

import (
	"os"

	"log/slog"

	"github.com/parkingwang/igo/internal/config"
)

var defaultConfig config.Provider

func SetConfig(path string) {
	c, err := config.LoadConfig(path)
	if err != nil {
		slog.Error("load config failed", err)
		os.Exit(1)
	}
	// 设置默认值
	c.SetDefault("app.name", "myservice")
	c.SetDefault("app.version", "0.0.1")
	defaultConfig = c
}

// Conf 返回默认配置文件
func Conf() config.Provider {
	if defaultConfig == nil {
		panic("default config nil")
	}
	return defaultConfig
}
