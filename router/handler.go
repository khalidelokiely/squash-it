package router

import (
	"context"
	"encoding/json"
	"net/http"
)

type RequestContext struct {
	Writer  http.ResponseWriter
	Request *http.Request
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

type RouteHandler func(ctx context.Context, c *RequestContext)
