package web

import (
	"github.com/go-playground/validator/v10"
	"github.com/parkingwang/igo/pkg/http/web/oas"
)

type option struct {
	render          Renderer
	dumpRequestBody bool
	addr            string
	routes          Routes
	docInfo         *oas.DocInfo
	bind            *validator.Validate
	pprof           bool
}

func defaultOption() *option {
	v := validator.New()
	v.SetTagName("binding") // 兼容gin
	return &option{
		addr:   ":8080",
		render: DefaultRender,
		routes: make([]*routeInfo, 0),
		bind:   v,
		pprof:  true,
	}
}

type Option func(*option)

// WithResponseRender 自定义响应输出
func WithResponseRender(r Renderer) Option {
	return func(opt *option) {
		opt.render = r
	}
}

// WithDumpRequestBody 是否输出请求体
func WithDumpRequestBody(o bool) Option {
	return func(opt *option) {
		opt.dumpRequestBody = o
	}
}

func WithAddr(addr string) Option {
	return func(o *option) {
		o.addr = addr
	}
}

func WithOpenAPI(info *oas.DocInfo) Option {
	return func(o *option) {
		o.docInfo = info
	}
}

func WithPProf(o bool) Option {
	return func(opt *option) {
		opt.pprof = o
	}
}
