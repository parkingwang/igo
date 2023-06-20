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
* ~~定时任务 [cron](https://github.com/robfig/cron) 计划支持~~


## 接口服务

基于gin实现的类RPC编码风格的restful服务，可方便的再rpc和传统的gin.handlefunc之前切换.

特性

* 支持链路追踪包含日志关联
* 使用RPC风格编码restful接口
* 自动生成接口文档 (openapi 3.0)
* 日志敏感词过滤
* 支持原始http/gin风格，解决rpc模式无法处理如websocket等需求问题


### 路由方法

标准的`RPC`风格

建议使用此风格，此风格对自动创建文档并做一些列内置的处理，帮你减少恨过工作


```go
func(context.Context, in *structRequest) (out *structResponse, err error)
```

定义

* ctx 上下文关联
* in 入参 必须是结构体指针  如果没有参数 可以使用 *web.Empty 内置类型 
* out 结果 必须是结构体指针

不带固定响应
> 有些情况下 不需要响应固定的结构 比如SSE服务器推送技术 或者websocket

```go
func(context.Context, in *structRequest) error
```

### 如何使用

```go
srv := app.CreateWebServer()
route := srv.Router()
route.Get("/user/:id", getUser)

type GetUserOption struct{
    // 通过 tag 标记此参数是url部分的变量
    ID string `uri:"id" comment:"用户的id"`
}

type UserInfo struct{
    ...
}

// getUser 这里的注释会在文档中显示
func getUser(ctx context.Context, in *GetUserOption) (*UserInfo, error){
    // 通过ctx 日志可以携带trace信息
    slog.InfoCtx("get user info", "id",in.ID)
    // 通过codes.New 返回error
    return nil, codes.NewBadRequest("some err")
}

```

### 绑定请求 / 验证

和gin保持一致  可以绑定query，header，form表单，json请求体，url参数等

> **query绑定 使用tag:jquery**
> json gin默认使用form tag进行绑定且仅对get请求有效 为了更好的通用性 这里使用tag:query进行绑定


验证则遵循 gin风格 

```go
type QueryRequestDemo struct{
    Field1   string `json:"field" binding:"required"`
    UrlParam string `uri:"id"`
}
```

### 使用原始`gin`风格

```go
srv := app.CreateWebServer()
r := srv.GinEngine() // 替换掉 srv.Router()
route.GET("/user/:id", func(*gin.Context){
    // todo
})

```

### 在`RPC`风格里使用`gin.Context`

```go
func hello(ctx context.Context, in *structRequest) error {
    ginCtx,ok:=web.GinContext(ctx)
    if ok{
        ginCtx.String(200,"hello")
    }
    return nil
}
```



## 链路追踪和日志

内部使用slog做为日志系统，为了进行链路关联 请使用slog的Ctx方法返回日志

默认使用框架的内的`database/redis/http`自带`trace` 请使用带有`context.Context`的方法进行操作

```go
func hello(ctx context.Context){
    slog.InfoCtx(ctx,"msg。。。。", "field_key","field_value")
}

```


