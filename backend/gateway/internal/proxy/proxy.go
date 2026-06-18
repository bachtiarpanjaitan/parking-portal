// Package proxy wires Gin routes to a reverse-proxy that forwards to a
// backend service URL. The proxy strips the gateway's prefix and forwards
// the original request body + headers.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// New builds a Gin handler that reverse-proxies to the given target URL.
// It preserves query strings, request body, and most headers (except
// hop-by-hop ones). The Authorization header is also forwarded so
// downstream services can validate the JWT.
func New(target string, timeout time.Duration) (gin.HandlerFunc, error) {
	tu, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("parse target %q: %w", target, err)
	}
	rp := httputil.NewSingleHostReverseProxy(tu)
	rp.Transport = &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	rp.ErrorHandler = func(rw http.ResponseWriter, r *http.Request, err error) {
		// The downstream service is unreachable. Return a JSON envelope so
		// the client always gets a consistent error shape.
		if errors.Is(err, context.DeadlineExceeded) {
			rw.WriteHeader(http.StatusGatewayTimeout)
		} else {
			rw.WriteHeader(http.StatusBadGateway)
		}
		_, _ = rw.Write([]byte(`{"success":false,"error":{"code":"UPSTREAM_UNREACHABLE","message":"upstream service is unreachable"}}`))
	}
	return func(c *gin.Context) {
		// Use a per-request timeout derived from the configured value.
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		rp.ServeHTTP(c.Writer, c.Request)
	}, nil
}
