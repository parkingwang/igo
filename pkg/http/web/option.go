package web

type option struct {
	render          Renderer
	dumpRequestBody bool
	addr            string
	routesInfo      []routeInfo
}

func defaultOption() *option {
	return &option{
		addr:       ":8080",
		render:     DefaultRender,
		routesInfo: make([]routeInfo, 0),
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

// WithDoc 启用文档
// func WithDoc(opt *ApiDocOption) Option {
// 	return func(o *option) {
// 		o.doc = opt
// 	}
// }
