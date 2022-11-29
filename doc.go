package igo

import "runtime/debug"

type AppInfo struct {
	// app name
	Name string
	// 描述
	Description string
	// 版本号
	Version string
}

func getVCSVersion() string {
	info, ok := debug.ReadBuildInfo()
	if ok {
		for _, v := range info.Settings {
			if v.Key == "vcs.revision" {
				return v.Value
			}
		}
	}
	return ""
}
