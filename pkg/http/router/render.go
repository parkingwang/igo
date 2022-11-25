package router

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/parkingwang/igo/pkg/http/code"
	"gorm.io/gorm"
)

// Renderer 渲染响应
type Renderer func(*gin.Context, any, error)

// DefaultRender 默认渲染函数
func DefaultRender(ctx *gin.Context, data any, err error) {
	if err != nil {
		var e *code.CodeError
		if errors.As(err, &e) {
			ctx.JSON(e.Code, gin.H{"message": e.Message})
		} else {
			code := http.StatusInternalServerError
			if errors.Is(err, gorm.ErrRecordNotFound) {
				code = http.StatusNotFound
			}
			ctx.JSON(code, gin.H{"message": err.Error()})
		}
	} else {
		if data != nil {
			ctx.JSON(http.StatusOK, data)
			return
		}
	}
}

func warpRender(opt *option, ctx *gin.Context, data any, err error) {
	if err != nil {
		var rawErr *code.CodeError
		if errors.As(err, &rawErr) {
			ctx.Set("gin.response.err", rawErr.Message)
		} else {
			ctx.Set("gin.response.err", err.Error())
		}
	}
	// 输出 response ？
	// 无法确定结果集大小 贸然输出可能会导致造成大量的垃圾日志
	// slog.Ctx(ctx).Info("gin.response.data", slog.Any("data", data))
	// 渲染结构
	opt.render(ctx, data, err)
}
