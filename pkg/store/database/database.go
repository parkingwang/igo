package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

var dbs map[string]*gorm.DB

const defaultName = "default"

// Register 注册默认数据库
// 当只有一个数据库的时候推荐使用
func Register(dsn string, opts ...Option) error {
	return RegisterByName(defaultName, dsn, opts...)
}

// RegisterByName 按名称注册数据库
// 适合同时需要操作多个数据库
func RegisterByName(name, dsn string, opts ...Option) error {
	if _, ok := dbs[name]; ok {
		return fmt.Errorf("db %s alreay register", name)
	}
	db, err := gorm.Open(
		mysql.Open(dsn),
		&gorm.Config{Logger: &emptyLogger{}},
	)
	if err != nil {
		return err
	}
	// 启动opentelemetry
	if err := db.Use(tracing.NewPlugin(tracing.WithoutMetrics())); err != nil {
		return err
	}
	for _, apply := range opts {
		if err := apply(db); err != nil {
			return err
		}
	}
	return nil
}

// Get 获取数据库
func Get(ctx context.Context, name ...string) *gorm.DB {
	var n string
	if len(name) == 0 || name[0] == "" {
		n = defaultName
	} else {
		n = name[0]
	}
	db, ok := dbs[n]
	if ok {
		return db.WithContext(ctx)
	}
	panic(fmt.Sprintf("db %s not registor", n))
}

// Option 数据库的一些配置
type Option func(*gorm.DB) error

func WithMaxOpenConns(n int) Option {
	return func(db *gorm.DB) error {
		d, err := db.DB()
		if err != nil {
			return err
		}
		d.SetMaxIdleConns(n)
		return nil
	}
}

func WithMaxIdleConns(n int) Option {
	return func(db *gorm.DB) error {
		d, err := db.DB()
		if err != nil {
			return err
		}
		d.SetMaxIdleConns(n)
		return nil
	}
}

func WithConnMaxIdleTime(n time.Duration) Option {
	return func(db *gorm.DB) error {
		d, err := db.DB()
		if err != nil {
			return err
		}
		d.SetConnMaxIdleTime(n)
		return nil
	}
}

func WithAutoMigrate(dst ...interface{}) Option {
	return func(d *gorm.DB) error {
		return d.AutoMigrate(dst...)
	}
}
