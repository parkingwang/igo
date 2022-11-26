package igo

import (
	"flag"
	"os"

	"github.com/parkingwang/igo/internal/config"
	"golang.org/x/exp/slog"
)

var defaultConfig config.Provider

func init() {
	var cpath string
	flag.StringVar(&cpath, "c", "config.toml", "配置文件路径")
	flag.Parse()
	c, err := config.LoadConfig(cpath)
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
