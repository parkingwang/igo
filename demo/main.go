package main

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/parkingwang/igo"
	"github.com/parkingwang/igo/pkg/http/code"
	"github.com/parkingwang/igo/pkg/http/router"
	"golang.org/x/exp/slog"
)

type Something struct {
	Value string
}

func main() {

	app := igo.New("myServer")

	app.Provide(
		// 添加一个构造函数 有些地方依赖它
		func() *Something {
			return &Something{Value: "nihao"}
		},
	)

	app.Run(
		func(s *Something) igo.Servicer {
			return &ticker{value: s.Value}
		},

		func() igo.Servicer {
			r := router.New(
				router.WithAddr(":8084"),
				router.WithDumpRequestBody(true),
			)
			initRoutes(r.RPCRouter())
			return r
		},
	)
}

func initRoutes(r router.Router) {
	r.Comment("获取用户列表").
		Get("/", middleGinHandler, listUser)
	r.Comment("获知指定id的用户信息").
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

func listUser(ctx context.Context, in *router.Empty) ([]UserInfo, error) {
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
