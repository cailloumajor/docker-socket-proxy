package main

import (
	"context"
	"fmt"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/go-kit/log"
)

func checkSocket(d *net.Dialer, sf string) error {
	c, err := d.Dial("unix", sf)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			panic(err)
		}
	}()

	return nil
}

func rewrite(pr *httputil.ProxyRequest) {
	pr.SetURL(&url.URL{
		Scheme: "http",
		Host:   "localhost",
	})
	pr.Out.Host = "docker-socket"
	pr.Out.Header.Set("User-Agent", "docker-socket-proxy/"+projectVersion)
}

// NewProxy creates a new Docker socket proxy on the given socket file.
func NewProxy(sf string, l log.Logger) (*httputil.ReverseProxy, error) {
	d := &net.Dialer{
		Timeout: 1 * time.Second,
	}

	if err := checkSocket(d, sf); err != nil {
		return nil, fmt.Errorf("error checking socket: %w", err)
	}

	dc := func(ctx context.Context, _, _ string) (net.Conn, error) {
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
	}, nil
}
