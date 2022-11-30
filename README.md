# igo

集成链路追踪的后端服务框架 [例子](/example/main.go)

依赖库
* web框架 [gin](github.com/gin-gonic/gin)
* 数据库 (mysql,pgsql,mssql,clickhouse) [gorm](https://gorm.io)
* 配置文件 [viper](https://github.com/spf13/viper)
* 依赖注入 [fx](https://uber-go.github.io/fx)
* 链路追踪 [opentelemetry](https://opentelemetry.io)
* 缓存 
    * redis [go-redis](https://redis.uptrace.dev)
* 日志 [slog](https://go.googlesource.com/proposal/+/master/design/56345-structured-logging.md)
* 消息系统
    * rabbitmq [rabbitmq/amqp091-go](github.com/rabbitmq/amqp091-go)
* 定时任务 [cron](https://github.com/robfig/cron) todo


接口服务

基于gin实现的类RPC编码风格的restful服务

* 支持链路追踪包含日志关联
* 使用RPC风格编码restful接口
* 自动生成接口文档 (openapi 3.0)
* 日志敏感词过滤
* 支持原始http/gin风格，解决rpc模式无法处理如websocket等需求问题




