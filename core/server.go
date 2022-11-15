package core

import "context"

// StartServerfunc 启动服务
// 如果上层退出或取消 则内部需要平滑退出
type StartServerfunc func(context.Context) error
