package igo

import "github.com/parkingwang/igo/core/config"

var defaultConfig config.Provider

// SetConfig 设置配置文件路径
// 基于viper支持 json,toml,ini,yaml等类型
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
