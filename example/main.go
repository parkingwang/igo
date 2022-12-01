package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/parkingwang/igo"
	"github.com/parkingwang/igo/pkg/http/code"
	"github.com/parkingwang/igo/pkg/http/web"
	"golang.org/x/exp/slog"
)

// Something 你好
type Something struct {
	Value string
}

var info = igo.AppInfo{
	Name:        "myname",
	Description: "这是一个演示",
}

func main() {

	app := igo.New(info)

	app.Provide(
		// 添加一个构造函数 有些地方依赖它
		// 具体可以参考go-uber/fx
		func() *Something {
			return &Something{Value: "nihao"}
		},
	)

	app.Run(
		// 简单的定时器服务示例
		// 需要实现 igo.Servicer
		// 依赖 Provide 提供 *Something
		func(s *Something) igo.Servicer {
			return &ticker{value: s.Value}
		},
		// web服务
		func() igo.Servicer {
			srv := app.CreateWebServer()
			initRoutes(srv)
			return srv
		},
	)
}

func initRoutes(srv *web.Server) {

	r := srv.Router()

	r.Get("/", Hello)
	user := r.Group("/user")
	user.Comment("user object")
	user.Get("/", middleGinHandler, ListUser)
	user.Get("/:id", GetUser).Comment("测试用 id 可以是 1,2,3,4 试试换成不同的值看看")
	user.Post("/:id/add", CreateUser)

}

// UserInfo 用户信息
type UserInfo struct {
	ID   int    `json:"id" uri:"id"`
	Name string `json:"name" form:"name" comment:"备注再这里"`
}

// UserInfoListResponse 用户信息列表
type UserInfoListResponse struct {
	Items []UserInfo `json:"items"`
}

// UserInfoListRequest 请求
type UserInfoListRequest struct {
	Page     int    `form:"page" binding:"gte=0" comment:"第几页"`
	PageSize int    `form:"pageSize" binding:"gte=0,lte=100" comment:"每页条数"`
	Keyword  string `form:"keyword" comment:"按指定关键字查询"`
}

func ListUser(ctx context.Context, in *UserInfoListRequest) (*UserInfoListResponse, error) {
	log := slog.Ctx(ctx)
	log.Info("get users", "count", len(userlist))
	if v := ctx.Value("value"); v != nil {
		log.Info("get middle value", "value", v)
	}
	return &UserInfoListResponse{Items: userlist}, nil
}

var userlist = []UserInfo{
	{1, "afocus"},
	{2, "umiko"},
	{3, "tom"},
	{4, "jack"},
}

type UserIDReq struct {
	ID int `uri:"id" binding:"required" comment:"用户id"`
}

// GetUser 获取单个用户
func GetUser(ctx context.Context, in *UserIDReq) (*UserInfo, error) {
	for _, user := range userlist {
		if user.ID == in.ID {
			return &user, nil
		}
	}
	return nil, code.NewNotfoundError("user not found")
}

// Hello 通过使用gin.Context 可以突破rpc风格上的使用限制
func Hello(ctx context.Context, in *web.Empty) error {
	c, ok := web.GinContext(ctx)
	if ok {
		fmt.Println(c.Request.URL.Hostname())
		fmt.Println(c.Request.Host)
		c.String(http.StatusOK, "hello,world")
	}
	return nil
}

func CreateUser(ctx context.Context, in *UserInfo) (*UserInfo, error) {
	return in, nil
}

// 一个gin风格的中间件
func middleGinHandler(c *gin.Context) {
	log := slog.Ctx(c)
	// 传递值
	c.Set("value", "123")
	log.Info("start")
	c.Next()
	log.Info("end")
}

// ////////////////////
// 自定义一个服务 实现igo.Servicer接口
type ticker struct {
	t     *time.Ticker
	value string
	log   *slog.Logger
}

func (tk *ticker) Start(ctx context.Context) error {
	log := slog.Ctx(ctx).With("type", "ticker")
	log.Info("start")
	tk.log = log
	tk.t = time.NewTicker(time.Second * 3)

	var i int
	go func() {
		for range tk.t.C {
			log.Info(tk.value, "index", i)
			i++
		}
	}()
	return nil
}

func (tk *ticker) Stop(ctx context.Context) error {
	tk.log.Info("ticker end")
	tk.t.Stop()
	return nil
}
