package router

import (
	"fmt"
)

type RouteGroup struct {
	prefix      string
	middlewares []HandlerFunc
	router      *Router
}

// Group creates a grouped set of routes under a static prefix. All Group Middleware are executed first before individual
// middleware
func (r *Router) Group(prefix string, middleware ...HandlerFunc) *RouteGroup {
	middlewares := make([]HandlerFunc, len(middleware))
	copy(middlewares, middleware)

	return &RouteGroup{
		prefix:      prefix,
		middlewares: middlewares,
		router:      r,
	}
}

// Use add middleware(s) to the group
func (g *RouteGroup) Use(middleware ...HandlerFunc) {
	g.middlewares = append(g.middlewares, middleware...)
}

func (g *RouteGroup) GET(path string, handler HandlerFunc, mw ...HandlerFunc) {
	fullPath := g.getGroupPath(path)
	g.router.GET(fullPath, handler, g.getMiddlewareChain(mw...)...)
}

func (g *RouteGroup) POST(path string, handler HandlerFunc, mw ...HandlerFunc) {
	fullPath := g.getGroupPath(path)
	g.router.POST(fullPath, handler, g.getMiddlewareChain(mw...)...)
}

func (g *RouteGroup) getGroupPath(path string) string {
	return fmt.Sprintf("%s%s", g.prefix, path)
}

func (g *RouteGroup) getMiddlewareChain(mw ...HandlerFunc) []HandlerFunc {
	// Allocate full capacity upfront to avoid intermediate memory copies
	allMiddlewares := make([]HandlerFunc, 0, len(g.middlewares)+len(mw))
	allMiddlewares = append(allMiddlewares, g.middlewares...)
	allMiddlewares = append(allMiddlewares, mw...)
	return allMiddlewares
}
