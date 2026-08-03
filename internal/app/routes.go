package app

import (
	"squash-it/internal/rate"
	"squash-it/internal/router"
)

func NewRoutes(r *router.Router, h *URLShortenHandler, limiter rate.Limiter) {
	g := r.Group("", RateLimiterMiddleware(limiter))
	{
		g.POST("/encode", h.EncodeURL)
		g.POST("/decode", h.DecodeURL)
	}

	r.GET("/{pathHash}", h.VisitURL)
}
