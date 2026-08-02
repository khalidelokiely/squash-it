package main

import (
	"context"
	"fmt"
	"net/http"
	"squash-it/router"
)

type URLEncodeDTO struct {
	LongURL string `json:"long_url"`
}

type URLDecodeDTO struct {
	PathHash string `json:"path_hash"`
}

type URLShortenHandler struct {
	svc *URLService
}

func NewURLShortenHandler(svc *URLService) *URLShortenHandler {
	return &URLShortenHandler{
		svc: svc,
	}
}

func (h *URLShortenHandler) EncodeURL(ctx context.Context, c *router.RequestContext) {
	var urlDTO URLEncodeDTO

	if err := c.BindAndValidate(&urlDTO); err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	hash, err := h.svc.ShortenURL(ctx, urlDTO.LongURL)

	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"short_url": fmt.Sprintf("%s/%s", c.Request.Host, hash),
	})
	return
}

func (h *URLShortenHandler) DecodeURL(ctx context.Context, c *router.RequestContext) {
	var pathHashDTO URLDecodeDTO
	if err := c.BindAndValidate(&pathHashDTO); err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
	}

	longURL, err := h.svc.GetURLFromPathHash(ctx, pathHashDTO.PathHash)

	if err != nil {
		c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, URLEncodeDTO{
		LongURL: fmt.Sprintf("%s", longURL),
	})

	return
}

func (h *URLShortenHandler) VisitURL(ctx context.Context, c *router.RequestContext) {
	hashToken := c.Param("hashToken")

	longURL, err := h.svc.GetURLFromPathHash(ctx, hashToken)

	if err != nil {
		c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "invalid hash token",
		})
		return
	}

	c.Redirect(http.StatusFound, longURL)
	return
}
