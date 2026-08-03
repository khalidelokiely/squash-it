package app

import "squash-it/internal/router"

func NewRoutes(r *router.Router, h URLShortenHandler) {
	r.POST("/encode", h.EncodeURL)
	r.POST("/decode", h.DecodeURL)
	r.GET("/{hashToken}", h.VisitURL)
}
