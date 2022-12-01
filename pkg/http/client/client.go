package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/sony/gobreaker"
)

type Client struct {
	opt Option
	// 熔断器
	breaker *gobreaker.CircuitBreaker
}

type Option struct {
	// 默认使用 http.DefaultClient
	Client *http.Client
	// 解析相应 **必须包含**
	// 无需手动response.Body.Close() 会自动调用
	// func(res *http.Response, out any) error {
	// 	  dec:=json.NewDecode(res.Body)
	//    return dec.Decode(out)
	// }
	ParseResponse func(*http.Response, any) error
	// 修改请求
	// 比如统一添加auth 或 签名认证等信息
	ModfityRequest func(*http.Request)
	// 基础url
	BaseURL string
	// 熔断器配置
	BreakerSetting *gobreaker.Settings
}

func NewClient(opt Option) (*Client, error) {
	if opt.ParseResponse == nil {
		return nil, errors.New("option ParseResponse must")
	}
	if opt.Client == nil {
		opt.Client = http.DefaultClient
	}
	client := &Client{opt: opt}
	if opt.BreakerSetting != nil {
		client.breaker = gobreaker.NewCircuitBreaker(*opt.BreakerSetting)
	}
	return client, nil
}

func MustClient(opt Option) *Client {
	m, err := NewClient(opt)
	if err != nil {
		panic(err)
	}
	return m
}

// Do 发送请求
// 有些情况下比较特殊 需要完全自定义request
// r,_:=http.NewRequestWithContext(ctx...)
// client.Do(r, &responseData)
func (c *Client) Do(r *http.Request, out any) error {
	if c.opt.ModfityRequest != nil {
		c.opt.ModfityRequest(r)
	}
	var response *http.Response
	if c.breaker != nil {
		ret, err := c.breaker.Execute(func() (interface{}, error) {
			return c.opt.Client.Do(r)
		})
		if err != nil {
			return err
		}
		response = ret.(*http.Response)
	} else {
		var err error
		response, err = c.opt.Client.Do(r)
		if err != nil {
			return err
		}
	}
	defer response.Body.Close()
	return c.opt.ParseResponse(response, out)
}

func (c *Client) Get(url string) *request {
	return c.create(http.MethodGet, url)
}

func (c *Client) Post(url string) *request {
	return c.create(http.MethodPost, url)
}

func (c *Client) Put(url string) *request {
	return c.create(http.MethodPut, url)
}

func (c *Client) Patch(url string) *request {
	return c.create(http.MethodPatch, url)
}

func (c *Client) Delete(url string) *request {
	return c.create(http.MethodDelete, url)
}

type request struct {
	m      *Client
	url    string
	method string
	header map[string]string
	body   io.Reader
}

var requestPool = sync.Pool{
	New: func() any {
		return &request{}
	},
}

func (c *Client) create(method, uri string) *request {
	req := requestPool.Get().(*request)
	req.method = method
	req.url = uri
	req.header = make(map[string]string)
	req.m = c
	return req
}

func (r *request) Header(kvs ...string) *request {
	if len(kvs)/2 != 0 {
		kvs = append(kvs, "")
	}
	if r.header == nil {
		r.header = make(map[string]string)
	}
	for i := 0; i < len(kvs); i += 2 {
		r.header[kvs[i]] = kvs[i+1]
	}
	return r
}

// Body request body
// eq：string/[]bytes/io.Reader/url.Values/any(tojson)
func (r *request) Body(a any) *request {
	if a == nil {
		return r
	}
	switch v := a.(type) {
	case string:
		r.body = strings.NewReader(v)
	case []byte:
		r.body = bytes.NewReader(v)
	case url.Values:
		r.body = strings.NewReader(v.Encode())
	case io.Reader:
		r.body = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
		} else {
			r.body = bytes.NewReader(b)
		}
	}
	return r
}

// Do 执行请求 一旦执行 则request对象变为无效 请勿再次使用
// out 相应的对象 需要调用option.ParseResponse 处理
func (r *request) Do(ctx context.Context, out any) error {
	defer func() {
		r.body = nil
		r.m = nil
		requestPool.Put(r)
	}()
	url := r.m.opt.BaseURL + r.url
	req, err := http.NewRequestWithContext(ctx, r.method, url, r.body)
	if err != nil {
		return err
	}
	for k, v := range r.header {
		req.Header.Add(k, v)
	}
	return r.m.Do(req, out)
}
