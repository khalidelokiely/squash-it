package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"squash-it/internal/router"
)

// TODO[SCALING - Observability]: Add span tracer to every handler + metrics for p99

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

// EncodeURL binds to a URLEncodeDTO and passes it to URLService.CreateURL
// returns
//
//	400 Bad Request: If DTO is deformed and can't be unmarshaled into the URLDecodeDTO Struct
//	500 Internal Server Error: Upon failures beyond lookups
//	201 Created: Upon successful shortening of the LongURL.
func (h *URLShortenHandler) EncodeURL(ctx context.Context, c *router.RequestContext) {
	var urlDTO URLEncodeDTO

	if err := c.BindAndValidate(&urlDTO); err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	hash, err := h.svc.CreateURL(ctx, urlDTO.LongURL)

	if err != nil {
		if errors.Is(err, ErrInvalidURL) {
			c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, map[string]interface{}{
		"short_url": fmt.Sprintf("%s/%s", c.Request.Host, hash),
	})
	return
}

// DecodeURL binds to a URLDecodeDTO and passes it to URLService.GetURLFromHash
// returns
//
//	400 Bad Request: If DTO is deformed and can't be unmarshaled into the URLDecodeDTO Struct
//	404 Not Found: Not Found if pathHash is invalid
//	500 Internal Server Error: Upon failures beyond lookups
//	200 OK + URLEncodeDTO: Upon successful lookup
func (h *URLShortenHandler) DecodeURL(ctx context.Context, c *router.RequestContext) {
	var pathHashDTO URLDecodeDTO
	if err := c.BindAndValidate(&pathHashDTO); err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
	}

	longURL, err := h.svc.GetURLFromHash(ctx, pathHashDTO.PathHash)

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

	c.JSON(http.StatusOK, URLEncodeDTO{
		LongURL: fmt.Sprintf("%s", longURL),
	})

	return
}

// VisitURL takes a pathHash named parameter and redirects to original / long url.
// returns
//
//	404 Not Found: Not Found if pathHash is invalid
//	500 Internal Server Error: Upon failures beyond lookups
//	302 Found + Location Header: Upon successful lookup. 302 is intentional to prevent permanent browser caching.
func (h *URLShortenHandler) VisitURL(ctx context.Context, c *router.RequestContext) {
	hashToken := c.Param("pathHash")

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
