package main_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	dsp "github.com/cailloumajor/docker-socket-proxy"
	"github.com/cailloumajor/docker-socket-proxy/internal/testutils"
)

type logWriter struct {
	tb testing.TB
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.tb.Helper()
	w.tb.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}

func newTestLogger(tb testing.TB) *slog.Logger {
	return slog.New(slog.NewTextHandler(&logWriter{tb}, &slog.HandlerOptions{Level: slog.LevelError}))
}

type testSocketHandler []http.Request

func (h *testSocketHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	*h = append(*h, *req.Clone(context.Background()))
	rw.Header().Set("Test-Response", "socket-response")
	fmt.Fprintln(rw, "socket response")
}

func TestNewProxyError(t *testing.T) {
	sd := t.TempDir()

	sf := filepath.Join(sd, "test.sock")

	_, err := dsp.NewProxy(sf, slog.New(slog.DiscardHandler))

	if msg := testutils.AssertError(t, err, true); msg != "" {
		t.Error(msg)
	}
}

func TestProxyServeHTTP(t *testing.T) {
	sd := t.TempDir()

	sf := filepath.Join(sd, "test.sock")
	sl, err := net.Listen("unix", sf)
	if err != nil {
		t.Fatal("error creating socket for testing")
	}
	var sh testSocketHandler
	ss := &httptest.Server{
		Listener: sl,
		Config: &http.Server{
			Handler: &sh,
		},
	}
	ss.Start()
	defer ss.Close()

	p, err := dsp.NewProxy(sf, newTestLogger(t))
	if err != nil {
		t.Fatalf("unexpected NewProxy error: %v", err)
	}

	req := httptest.NewRequest("GET", "/some/endpoint", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	// Assertions about the response obtained by the client.
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("response status: want %d, got %d", want, got)
	}
	if got, want := resp.Header.Get("Test-Response"), "socket-response"; got != want {
		t.Errorf("response `Test-Response` header: want %q, got %q", want, got)
	}
	if got, want := string(body), "socket response\n"; got != want {
		t.Errorf("response body: want %q, got %q", want, got)
	}

	// Assertions about the request obtained by the socket.
	if got, want := len(sh), 1; got != want {
		t.Errorf("socket requests count, want %d, got %d", want, got)
	}
	if got, want := sh[0].Method, http.MethodGet; got != want {
		t.Errorf("socket request method: want %q, got %q", want, got)
	}
	if got, want := sh[0].URL.Scheme, ""; got != want {
		t.Errorf("socket request URL scheme: want %q, got %q", want, got)
	}
	if got, want := sh[0].URL.Host, ""; got != want {
		t.Errorf("socket request URL host: want %q, got %q", want, got)
	}
	if got, want := sh[0].URL.Path, "/some/endpoint"; got != want {
		t.Errorf("socket request URL path: want %q, got %q", want, got)
	}
	if got, want := sh[0].Header.Get("User-Agent"), "docker-socket-proxy/dev"; got != want {
		t.Errorf("socket request `User-Agent` header: want %q, got %q", want, got)
	}
	if got, want := sh[0].Host, "docker-socket"; got != want {
		t.Errorf("socket request host: want %q, got %q", want, got)
	}
}
