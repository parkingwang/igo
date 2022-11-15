package amqp

import (
	"context"

	"github.com/parkingwang/igo/core"
)

type Router interface {
	Handle(queue string, h MessageHandle)
}

func Server(f func(Router), opt ...Option) core.StartServerfunc {
	return func(ctx context.Context) error {
		sub := NewSub(opt...)
		f(sub)
		return sub.Wait(ctx)
	}
}
