package igo

import "github.com/parkingwang/igo/core/config"

func SetConfing(path string) {
	p, err := config.LoadConfig(path)
	if err != nil {
		panic(err)
	}
	defaultConfig = p
}

var defaultConfig config.Provider

func Conf() config.Provider {
	if defaultConfig == nil {
		panic("default config nil")
	}
	return defaultConfig
}
