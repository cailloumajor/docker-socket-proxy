package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

var apiPrefixRe = regexp.MustCompile(`^/v[0-9.]+`)

// RequestAccepter models a request accepter.
type RequestAccepter interface {
	AcceptRequest(*http.Request) bool
}

// AllowFilter represents a request-allowing filter.
type AllowFilter struct {
	Method string `toml:"method"` // HTTP method (exact match).
	Path   string `toml:"path"`   // Request path (pattern match).
}

// Validate does filter validation.
func (a *AllowFilter) Validate() error {
	if _, err := doublestar.Match(a.Path, ""); err != nil {
		return fmt.Errorf("error validating path pattern `%s`: %w", a.Path, err)
	}
	return nil
}

// AcceptRequest implements RequestAccepter.
func (a *AllowFilter) AcceptRequest(r *http.Request) bool {
	if r.Method != a.Method {
		return false
	}

	pr := apiPrefixRe.FindString(r.URL.Path)
	p := strings.TrimPrefix(r.URL.Path, pr)
	// Ignore the error here, a pattern check is required before using this method.
	if ok, _ := doublestar.Match(a.Path, p); !ok {
		return false
	}

	return true
}

// RequestAccepters represents a slice of request accepters.
type RequestAccepters []RequestAccepter

// AcceptRequest implements RequestAccepter.
func (ras RequestAccepters) AcceptRequest(r *http.Request) bool {
	for _, ra := range ras {
		if ra.AcceptRequest(r) {
			return true
		}
	}

	return false
}

// FilteringMiddleware wraps a handler and filters requests against rules.
type FilteringMiddleware struct {
	wrapped  http.Handler
	accepter RequestAccepter
	logger   *slog.Logger
}

// NewFilteringMiddleware creates a filtering middleware.
func NewFilteringMiddleware(h http.Handler, a RequestAccepter, l *slog.Logger) *FilteringMiddleware {
	return &FilteringMiddleware{h, a, l}
}

// ServeHTTP implements http.Handler.
func (f *FilteringMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logRequest := func(level slog.Level, msg string) {
		rg := slog.Group("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("from", r.RemoteAddr),
		)
		f.logger.LogAttrs(r.Context(), level, msg, rg)
	}

	logRequest(slog.LevelDebug, "request received")

	if r.Method == http.MethodHead || f.accepter.AcceptRequest(r) {
		f.wrapped.ServeHTTP(w, r)
	} else {
		logRequest(slog.LevelWarn, "request rejected")
		http.Error(w, progName+": request rejected", http.StatusForbidden)
	}
}
