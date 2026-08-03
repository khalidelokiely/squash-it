package router

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

const abortIndex int8 = 63

type HandlerFunc func(ctx context.Context, c *RequestContext)
type RequestContext struct {
	Writer    http.ResponseWriter
	Request   *http.Request
	handlers  []HandlerFunc
	currIndex int
}

func (r *RequestContext) JSON(statusCode int, body interface{}) {
	r.Writer.Header().Set("Content-Type", "application/json")

	r.Writer.WriteHeader(statusCode)

	if err := json.NewEncoder(r.Writer).Encode(body); err != nil {
		// Fallback error behavior if encoding fails
		http.Error(r.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (r *RequestContext) Redirect(status int, url string) {
	r.Writer.Header().Set("Location", url)
	r.Writer.WriteHeader(status)
}

func (r *RequestContext) BindAndValidate(target interface{}) error {
	decoder := json.NewDecoder(r.Request.Body)

	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	return nil
}

func (r *RequestContext) Param(name string) string {
	return r.Request.PathValue(name)
}

func (r *RequestContext) GetClientIP() string {
	// GetClientIP extracts the real user IP from the request.
	// 1. Check if the server is behind a reverse proxy.
	// Proxies pass the original user's IP in the X-Forwarded-For header.
	if xff := r.Request.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain a comma-separated list of IPs if there are multiple proxies.
		// The first IP in the list is always the original client.
		ips := strings.Split(xff, ",")
		clientIP := strings.TrimSpace(ips[0])

		// Basic validation: ensure it's a valid IP string
		if net.ParseIP(clientIP) != nil {
			return clientIP
		}
	}

	// Alternative proxy header (used by some proxies/load balancers like AWS/Cloudflare)
	if xRealIP := r.Request.Header.Get("X-Real-IP"); xRealIP != "" {
		if net.ParseIP(xRealIP) != nil {
			return xRealIP
		}
	}

	// Fallback to RemoteAddr if no proxy headers exist.
	host, _, err := net.SplitHostPort(r.Request.RemoteAddr)
	if err != nil {
		return r.Request.RemoteAddr
	}

	return host
}

// Next Executes the next handler func in chain
func (r *RequestContext) Next(c context.Context) {
	r.currIndex++
	for r.currIndex < len(r.handlers) {
		r.handlers[r.currIndex](c, r)
		r.currIndex++
	}
}

// Abort aborts the request. It stops where the Abort was called. If in a middleware no further requests including
// the handler are called.
func (r *RequestContext) Abort() {
	r.currIndex = 64
}
