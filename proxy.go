package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
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
func NewProxy(sf string, l *slog.Logger) (*httputil.ReverseProxy, error) {
	d := &net.Dialer{
		Timeout: 1 * time.Second,
	}

	if err := checkSocket(d, sf); err != nil {
		return nil, fmt.Errorf("error checking socket: %w", err)
	}

	dc := func(ctx context.Context, _, _ string) (net.Conn, error) {
		return d.DialContext(ctx, "unix", sf)
	}

	errorLog := slog.NewLogLogger(l.Handler(), slog.LevelError)

	return &httputil.ReverseProxy{
		Rewrite: rewrite,
		Transport: &http.Transport{
			DialContext: dc,
		},
		ErrorLog: errorLog,
	}, nil
}
