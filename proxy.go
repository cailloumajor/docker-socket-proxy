package main

import (
	"context"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/go-kit/log"
)

func rewrite(pr *httputil.ProxyRequest) {
	pr.SetURL(&url.URL{
		Scheme: "http",
		Host:   "localhost",
	})
	pr.Out.Host = "docker-socket"
	pr.Out.Header.Set("User-Agent", "docker-socket-proxy/"+projectVersion)
}

// NewProxy creates a new Docker socket proxy on the given socket file.
func NewProxy(sf string, l log.Logger) *httputil.ReverseProxy {
	dc := func(ctx context.Context, _, _ string) (net.Conn, error) {
		d := &net.Dialer{
			Timeout: 1 * time.Second,
		}
		return d.DialContext(ctx, "unix", sf)
	}

	const goReverseProxyPrefix = "go_reverse_proxy"
	rl := log.NewStdlibAdapter(
		log.With(l, "component", goReverseProxyPrefix),
		log.Prefix(goReverseProxyPrefix, false),
	)
	sl := stdlog.New(rl, goReverseProxyPrefix, stdlog.LstdFlags)

	return &httputil.ReverseProxy{
		Rewrite: rewrite,
		Transport: &http.Transport{
			DialContext: dc,
		},
		ErrorLog: sl,
	}
}
