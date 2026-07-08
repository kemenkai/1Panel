package proxy

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/1Panel-dev/1Panel/core/utils/platform"
)

var (
	LocalAgentProxy *httputil.ReverseProxy
)

func Init() {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	targetHost := "unix"
	transport := &http.Transport{
		ForceAttemptHTTP2:   false,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     30 * time.Second,
	}
	if platform.UseLocalAgentSocket() {
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", platform.LocalAgentSockPath)
		}
	} else {
		targetHost = platform.LocalAgentHTTPAddr
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", platform.LocalAgentHTTPAddr)
		}
	}
	LocalAgentProxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			if req.Header.Get("X-Forwarded-Proto") == "" {
				if req.TLS != nil {
					req.Header.Set("X-Forwarded-Proto", "https")
				} else {
					req.Header.Set("X-Forwarded-Proto", "http")
				}
			}
			if req.Header.Get("X-Forwarded-Host") == "" && req.Host != "" {
				req.Header.Set("X-Forwarded-Host", req.Host)
			}
			req.URL.Scheme = "http"
			req.URL.Host = targetHost
		},
		Transport: transport,
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			rw.WriteHeader(http.StatusBadGateway)
			_, _ = rw.Write([]byte("Bad Gateway: " + err.Error()))
		},
	}
}
