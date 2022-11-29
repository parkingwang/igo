package main

import (
	"context"
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

func main() {

	app := igo.New()

	app.Provide(
		// 添加一个构造函数 有些地方依赖它
		func() *Something {
			return &Something{Value: "nihao"}
		},
	)

	app.Run(
		// 简单的定时器服务示例
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

	r := srv.RPCRouter()

	r.Get("/", Hello)

	user := r.Group("/user")

	user.Get("/", middleGinHandler, listUser)
	user.Get("/:id", GetUser)

}

// UserInfo 用户信息
type UserInfo struct {
	ID   int    `json:"id"`
	Name string `json:"name" comment:"备注再这里"`
}

// UserInfoList 用户信息列表
type UserInfoList struct {
	Items []UserInfo `json:"items"`
}

// listUser 获取用户列表.
func listUser(ctx context.Context, in *web.Empty) (*UserInfoList, error) {
	log := slog.Ctx(ctx)
	log.Info("get users", "count", len(userlist))
	if v := ctx.Value("value"); v != nil {
		log.Info("get middle value", "value", v)
	}
	return &UserInfoList{Items: userlist}, nil
}

var userlist = []UserInfo{
	{1, "afocus"},
	{2, "umiko"},
	{3, "tom"},
	{4, "jack"},
}

type UserIDReq struct {
	ID   int    `json:"id" binding:"required"`
	Name string `json:"name"`
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

// Hello 一些方法.
func Hello(ctx context.Context, in *web.Empty) error {
	c, ok := web.RawContext(ctx)
	if ok {
		c.String(http.StatusOK, "hello,world")
	}
	return nil
}

func middleGinHandler(c *gin.Context) {
	log := slog.Ctx(c)

	// 传递值
	c.Set("value", "123")

	log.Info("start")
	c.Next()
	log.Info("end")
}

//////////////////////

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
