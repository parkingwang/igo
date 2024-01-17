package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

var dbs = make(map[string]*gorm.DB)

const (
	defaultName    = "default"
	DriverMysql    = "mysql"
	DriverPostgres = "postgres"
)

// Register 注册默认数据库
// 当只有一个数据库的时候推荐使用
func Register(dsn, driver string, opts ...Option) error {
	return RegisterByName(defaultName, dsn, driver, opts...)
}

// RegisterByName 按名称注册数据库
// 适合同时需要操作多个数据库
func RegisterByName(name, dsn, driver string, opts ...Option) error {
	if _, ok := dbs[name]; ok {
		return fmt.Errorf("db %s alreay register", name)
	}
	var dialect gorm.Dialector
	switch driver {
	case DriverPostgres:
		dialect = postgres.Open(dsn)
	default:
		dialect = mysql.Open(dsn)
	}

	db, err := gorm.Open(
		dialect,
		&gorm.Config{Logger: &tracelogger{}},
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
	dbs[name] = db
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
