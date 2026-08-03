package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"squash-it/internal/router"
)

var ErrInvalidURL = errors.New("invalid URL. URL must start with http or https")

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

	if !h.svc.isValidURL(urlDTO.LongURL) {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": ErrInvalidURL.Error(),
		})
		return
	}

	hash, err := h.svc.CreateURL(ctx, urlDTO.LongURL)

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

	longURL, err := h.svc.GetURLFromHash(ctx, pathHashDTO.PathHash)

	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, map[string]interface{}{
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

	longURL, err := h.svc.GetURLFromHash(ctx, hashToken)

	if err != nil {
		if errors.Is(err, ErrUnknownHash) {
			c.JSON(http.StatusNotFound, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if err := h.svc.UpdateClickCount(ctx, hashToken); err != nil {
		log.Printf("failed to update click count: %v", err)
	}

	c.Redirect(http.StatusFound, longURL)
	return
}
