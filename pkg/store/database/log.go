package database

import (
	"context"
	"time"

	"gorm.io/gorm/logger"
)

// emptyLogger 空log
// 禁用日志输出 日志已经在trace中集成
type emptyLogger struct{}

func (l *emptyLogger) LogMode(logger.LogLevel) logger.Interface              { return l }
func (l *emptyLogger) Info(ctx context.Context, s string, v ...interface{})  {}
func (l *emptyLogger) Warn(ctx context.Context, s string, v ...interface{})  {}
func (l *emptyLogger) Error(ctx context.Context, s string, v ...interface{}) {}
func (l *emptyLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
}
