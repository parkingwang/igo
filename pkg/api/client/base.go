package client

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func init() {
	// default 没有任何超时
	http.DefaultClient = NewHttpClient(0)
}

func NewHttpTransport() http.RoundTripper {
	return otelhttp.NewTransport(http.DefaultTransport)
}

func NewHttpClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: NewHttpTransport(),
		Timeout:   timeout,
	}
}
