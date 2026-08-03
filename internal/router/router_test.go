package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRouter_MiddlewareOrderAndAbort verifies that middleware executes sequentially
// and that calling Abort() halts execution immediately.
func TestRouter_MiddlewareOrderAndAbort(t *testing.T) {
	t.Parallel()

	var executionOrder []string

	m1 := func(ctx context.Context, c *RequestContext) {
		executionOrder = append(executionOrder, "m1_start")
		c.Next(ctx)
		executionOrder = append(executionOrder, "m1_end")
	}

	m2Abort := func(ctx context.Context, c *RequestContext) {
		executionOrder = append(executionOrder, "m2_abort")
		c.Abort()
		c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	handler := func(ctx context.Context, c *RequestContext) {
		executionOrder = append(executionOrder, "handler")
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	r := NewRouter()
	r.GET("/protected", handler, m1, m2Abort)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	r.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Verify handler was never reached due to Abort()
	wantOrder := []string{"m1_start", "m2_abort", "m1_end"}
	if len(executionOrder) != len(wantOrder) {
		t.Fatalf("execution order length = %d, want %d (%v)", len(executionOrder), len(wantOrder), executionOrder)
	}

	for i, step := range wantOrder {
		if executionOrder[i] != step {
			t.Errorf("step %d = %q, want %q", i, executionOrder[i], step)
		}
	}
}

// TestRouteGroup_PathAndMiddleware verifies route prefixing and group-level middleware inheritance.
func TestRouteGroup_PathAndMiddleware(t *testing.T) {
	t.Parallel()

	r := NewRouter()
	var groupMwCalled bool

	groupMw := func(ctx context.Context, c *RequestContext) {
		groupMwCalled = true
		c.Next(ctx)
	}

	v1 := r.Group("/api/v1", groupMw)
	v1.GET("/ping", func(ctx context.Context, c *RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"message": "pong"})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	rec := httptest.NewRecorder()

	r.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if !groupMwCalled {
		t.Errorf("expected group middleware to be executed")
	}
}

// TestRequestContext_GetClientIP verifies IP extraction across proxy headers and direct RemoteAddr.
func TestRequestContext_GetClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		headerKey   string
		headerValue string
		remoteAddr  string
		wantIP      string
	}{
		{
			name:        "Should Extract First IP From X-Forwarded-For",
			headerKey:   "X-Forwarded-For",
			headerValue: "203.0.113.195, 70.41.3.18, 150.172.238.178",
			remoteAddr:  "127.0.0.1:1234",
			wantIP:      "203.0.113.195",
		},
		{
			name:        "Should Fallback To X-Real-IP",
			headerKey:   "X-Real-IP",
			headerValue: "198.51.100.1",
			remoteAddr:  "127.0.0.1:1234",
			wantIP:      "198.51.100.1",
		},
		{
			name:       "Should Fallback To RemoteAddr Host",
			remoteAddr: "192.168.1.5:54321",
			wantIP:     "192.168.1.5",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.headerKey != "" {
				req.Header.Set(tt.headerKey, tt.headerValue)
			}

			ctx := &RequestContext{Request: req}
			if got := ctx.GetClientIP(); got != tt.wantIP {
				t.Errorf("GetClientIP() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

// TestRequestContext_BindAndValidate verifies JSON decoding and unknown field rejection.
func TestRequestContext_BindAndValidate(t *testing.T) {
	t.Parallel()

	type payload struct {
		URL string `json:"url"`
	}

	t.Run("Valid Payload", func(t *testing.T) {
		t.Parallel()
		body := bytes.NewBufferString(`{"url":"https://google.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		ctx := &RequestContext{Request: req}

		var p payload
		if err := ctx.BindAndValidate(&p); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.URL != "https://google.com" {
			t.Errorf("URL = %q, want %q", p.URL, "https://google.com")
		}
	})

	t.Run("Reject Unknown Fields", func(t *testing.T) {
		t.Parallel()
		body := bytes.NewBufferString(`{"url":"https://google.com", "unknown_field": true}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		ctx := &RequestContext{Request: req}

		var p payload
		if err := ctx.BindAndValidate(&p); err == nil {
			t.Errorf("expected error for unknown fields, got nil")
		}
	})
}

// TestRouter_Options verifies functional option configuration.
func TestRouter_Options(t *testing.T) {
	t.Parallel()

	r := NewRouter(
		WithHostPorts("9090"),
		WithReadTimeout(10*time.Second),
		WithWriteTimeout(10*time.Second),
	)

	if r.opts.Port != "9090" {
		t.Errorf("Port = %q, want %q", r.opts.Port, "9090")
	}
	if r.opts.ReadTimeout != 10*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", r.opts.ReadTimeout, 10*time.Second)
	}
}
