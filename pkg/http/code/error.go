package code

import (
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
)

type CodeError struct {
	Code    int
	Message string
}

func (e *CodeError) Error() string {
	return fmt.Sprintf("%d %s", e.Code, e.Message)
}

func NewCodeError(code int, msg string, args ...any) error {
	if code == 0 {
		code = http.StatusInternalServerError
	}
	return &CodeError{code, fmt.Sprintf(msg, args...)}
}

// NewBadRequestError 请求参数错误
func NewBadRequestError(v any) error {
	if fe, ok := v.(validator.ValidationErrors); ok {
		if len(fe) > 0 {
			// fmt.Println("structfield", fe[0].StructField())
			// fmt.Println("param", fe[0].Param())
			// fmt.Println("error", fe[0].Error())
			// fmt.Println("tag", fe[0].Tag())
			// fmt.Println("actualTag", fe[0].ActualTag())
			v = fmt.Sprintf("%s %s", fe[0].Error(), fe[0].Param())
		}
	}
	return &CodeError{
		http.StatusBadRequest,
		fmt.Sprintf("验证失败 %v", v),
	}
}

// NewUnauthorizedError 请求需要通过身份验证
func NewUnauthorizedError(v any) error {
	return &CodeError{
		http.StatusUnauthorized,
		fmt.Sprintf("%v", v),
	}
}

// NewForbiddenError 拒绝访问 即使通过了身份验证 （权限，未授权IP等）
func NewForbiddenError(v any) error {
	return &CodeError{
		http.StatusForbidden,
		fmt.Sprintf("%v", v),
	}
}

// NewNotfoundError 服务器上没有请求的资源。路径错误等。
func NewNotfoundError(v any) error {
	return &CodeError{
		http.StatusNotFound,
		fmt.Sprintf("%v", v),
	}
}
