package web

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/parkingwang/igo/pkg/http/code"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Renderer 渲染响应
type Renderer func(*gin.Context, any, error)

// DefaultRender 默认渲染函数
func DefaultRender(ctx *gin.Context, data any, err error) {
	if err != nil {
		var e *code.CodeError
		if !errors.As(err, &e) {
			e = &code.CodeError{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			}
		}
		span := trace.SpanFromContext(ctx)
		span.SetStatus(codes.Error, err.Error())
		resp := DefaultErrorResponse{
			Message: e.Message,
			TraceID: span.SpanContext().TraceID().String(),
		}
		ctx.JSON(e.Code, resp)
	} else {
		if data != nil {
			ctx.JSON(http.StatusOK, data)
			return
		}
	}
}

type DefaultErrorResponse struct {
	Message string `json:"message"`
	TraceID string `json:"traceid"`
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
