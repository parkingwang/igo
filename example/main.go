package main

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/parkingwang/igo"
	"github.com/parkingwang/igo/pkg/http/code"
	"github.com/parkingwang/igo/pkg/http/web"
	"golang.org/x/exp/slog"
)

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

	user := r.Comment("用户").
		Group("/user")
	user.Comment("获取用户列表").
		Get("/", middleGinHandler, listUser)
	user.Comment("获知指定id的用户信息").
		Get("/:id", getUser)
}

type UserInfo struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var userlist = []UserInfo{
	{1, "afocus"},
	{2, "umiko"},
	{3, "tom"},
	{4, "jack"},
}

func listUser(ctx context.Context, in *web.Empty) ([]UserInfo, error) {
	log := slog.Ctx(ctx)

	log.Info("get users", "count", len(userlist))

	if v := ctx.Value("value"); v != nil {
		log.Info("get middle value", "value", v)
	}

	return userlist, nil
}

type UserIDReq struct {
	ID int `uri:"id" binding:"required"`
}

func getUser(ctx context.Context, in *UserIDReq) (*UserInfo, error) {
	for _, user := range userlist {
		if user.ID == in.ID {
			return &user, nil
		}
	}
	return nil, code.NewNotfoundError("user not found")
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
