package ginserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/parkingwang/igo/pkg/http/code"
)

// Renderer 渲染响应
type Renderer func(*gin.Context, any, error)

// DefaultRender 默认渲染函数
func DefaultRender(ctx *gin.Context, data any, err error) {
	if err != nil {
		c := http.StatusInternalServerError
		sterr, ok := err.(*code.CodeError)
		if ok {
			c = sterr.Code
		}
		ctx.JSON(c, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, data)
}
