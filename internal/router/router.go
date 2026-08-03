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

func (r *Router) addRoute(method, path string, handler HandlerFunc, mw ...HandlerFunc) {

	currentHandler := r.wrapHandler(handler, mw...)

	muxFQPattern := fmt.Sprintf("%s %s", method, path)
	r.mux.Handle(muxFQPattern, currentHandler)
}

func (r *Router) wrapHandler(handler HandlerFunc, mw ...HandlerFunc) http.Handler {
	chain := make([]HandlerFunc, 0, len(mw)+1)
	chain = append(chain, mw...)
	chain = append(chain, handler)

	if len(chain) >= int(abortIndex) {
		panic(fmt.Sprintf("too many middlewares! framework limit is %d, got %d", abortIndex-1, len(chain)))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := &RequestContext{
			Writer:    w,
			Request:   r,
			handlers:  chain,
			currIndex: -1,
		}
		ctx.Next(r.Context())
	})
}

func (r *Router) GET(path string, handler HandlerFunc, mw ...HandlerFunc) {
	r.addRoute("GET", path, handler, mw...)
}
func (r *Router) POST(path string, handler HandlerFunc, mw ...HandlerFunc) {
	r.addRoute("POST", path, handler, mw...)
}

// TODO: implement remaining methods upon need

func (r *Router) PUT(path string, handler HandlerFunc) {

}
func (r *Router) PATCH(path string, handler HandlerFunc) {

}
func (r *Router) DELETE(path string, handler HandlerFunc) {

}
