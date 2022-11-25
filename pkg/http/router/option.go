package router

type option struct {
	render          Renderer
	dumpRequestBody bool
	addr            string
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
