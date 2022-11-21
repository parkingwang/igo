package code

import (
	"fmt"
	"net/http"
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
