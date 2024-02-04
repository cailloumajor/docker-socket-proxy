package main_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	dsp "github.com/cailloumajor/docker-socket-proxy"
	"github.com/cailloumajor/docker-socket-proxy/internal/testutils"
	"github.com/go-kit/log"
)

type mockedAccepter bool

func (m mockedAccepter) AcceptRequest(_ *http.Request) bool {
	return bool(m)
}

func TestAllowFilterAccept(t *testing.T) {
	cases := []struct {
		name         string
		filter       dsp.RequestAccepter
		expectAccept bool
	}{
		{
			name:         "EmptyAllowFilter",
			filter:       &dsp.AllowFilter{},
			expectAccept: false,
		},
		{
			name: "BadMethod",
			filter: &dsp.AllowFilter{
				Method: "POST",
				Path:   "/some/**",
			},
			expectAccept: false,
		},
		{
			name: "EmptyMethod",
			filter: &dsp.AllowFilter{
				Method: "",
				Path:   "/some/**",
			},
			expectAccept: false,
		},
		{
			name: "BadPath",
			filter: &dsp.AllowFilter{
				Method: "GET",
				Path:   "/other/**",
			},
			expectAccept: false,
		},
		{
			name: "EmptyPathPattern",
			filter: &dsp.AllowFilter{
				Method: "GET",
				Path:   "",
			},
			expectAccept: false,
		},
		{
			name: "Good",
			filter: &dsp.AllowFilter{
				Method: "GET",
				Path:   "/some/**",
			},
			expectAccept: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/v2.42/some/long/action/path",
				},
			}

			a := tc.filter.AcceptRequest(req)

			if got, want := a, tc.expectAccept; got != want {
				t.Errorf("request accepted: want %v, got %v", want, got)
			}
		})
	}
}

func TestAllowFilterValidate(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "BadPathPattern",
			path:        "[",
			expectError: true,
		},
		{
			name:        "Success",
			path:        "/*/some/**",
			expectError: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &dsp.AllowFilter{
				Path: tc.path,
			}

			err := f.Validate()

			if msg := testutils.AssertError(t, err, tc.expectError); msg != "" {
				t.Error(msg)
			}
		})
	}
}

func TestRequestAcceptersAccept(t *testing.T) {
	cases := []struct {
		name         string
		filters      dsp.RequestAccepter
		expectAccept bool
	}{
		{
			name:         "EmptySlice",
			filters:      dsp.RequestAccepters{},
			expectAccept: false,
		},
		{
			name:         "Rejected",
			filters:      dsp.RequestAccepters{mockedAccepter(false), mockedAccepter(false)},
			expectAccept: false,
		},
		{
			name:         "AcceptedFirst",
			filters:      dsp.RequestAccepters{mockedAccepter(true), mockedAccepter(false)},
			expectAccept: true,
		},
		{
			name:         "AcceptedSecond",
			filters:      dsp.RequestAccepters{mockedAccepter(false), mockedAccepter(true)},
			expectAccept: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := tc.filters.AcceptRequest(&http.Request{})

			if got, want := a, tc.expectAccept; got != want {
				t.Errorf("request accepted: want %v, got %v", want, got)
			}
		})
	}
}

func TestFilteringMiddleware(t *testing.T) {
	wh := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "success")
	})

	cases := []struct {
		name         string
		method       string
		accepter     dsp.RequestAccepter
		expectStatus int
		expectBody   string
	}{
		{
			name:         "Rejected",
			method:       http.MethodGet,
			accepter:     mockedAccepter(false),
			expectStatus: http.StatusForbidden,
			expectBody:   "docker-socket-proxy: request rejected\n",
		},
		{
			name:         "HeadRequest",
			method:       http.MethodHead,
			accepter:     mockedAccepter(false),
			expectStatus: http.StatusOK,
			expectBody:   "success\n",
		},
		{
			name:         "Accepted",
			method:       http.MethodGet,
			accepter:     mockedAccepter(true),
			expectStatus: http.StatusOK,
			expectBody:   "success\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fw := dsp.NewFilteringMiddleware(wh, tc.accepter, log.NewNopLogger())

			req := httptest.NewRequest(tc.method, "/", nil)
			w := httptest.NewRecorder()
			fw.ServeHTTP(w, req)

			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)

			if got, want := resp.StatusCode, tc.expectStatus; got != want {
				t.Errorf("response status code: want %d, got %d", want, got)
			}
			if got, want := string(body), tc.expectBody; got != want {
				t.Errorf("response body: want %q, got %q", want, got)
			}
		})
	}
}
