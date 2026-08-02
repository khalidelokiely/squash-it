package router

import (
	"fmt"
	"net/http"
	"time"
)

type Router struct {
	mux  *http.ServeMux
	opts *Options
}

// NewRouter creates a new router
func NewRouter(opts ...Option) *Router {
	options := &Options{
		Port:         "8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	for _, opt := range opts {
		opt.F(options)
	}

	return &Router{
		mux:  http.NewServeMux(),
		opts: options,
	}
}

func (r *Router) addRoute(method, path string, handler RouteHandler, mw ...Middleware) {

	currentHandler := r.wrapHandler(handler)

	for i := len(mw) - 1; i >= 0; i-- {
		currentHandler = mw[i](currentHandler)
	}

	muxFQPattern := fmt.Sprintf("%s %s", method, path)
	r.mux.Handle(muxFQPattern, currentHandler)
}

func (r *Router) wrapHandler(handler RouteHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := &RequestContext{
			Writer:  w,
			Request: r,
		}
		handler(r.Context(), ctx)
	})
}

func (r *Router) GET(path string, handler RouteHandler, mw ...Middleware) {
	r.addRoute("GET", path, handler, mw...)
}
func (r *Router) POST(path string, handler RouteHandler, mw ...Middleware) {
	r.addRoute("POST", path, handler, mw...)
}

// TODO: implement remaining methods upon need

func (r *Router) PUT(path string, handler RouteHandler) {

}
func (r *Router) PATCH(path string, handler RouteHandler) {

}
func (r *Router) DELETE(path string, handler RouteHandler) {

}
