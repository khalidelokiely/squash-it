package app

import (
	"context"
	"net/http"
	"squash-it/internal/rate"
	"squash-it/internal/router"
)

func RateLimiterMiddleware(limiter rate.Limiter) router.HandlerFunc {
	return func(ctx context.Context, c *router.RequestContext) {
		if limiter == nil {
			c.Next(ctx)
			return
		}

		clientIdentifier := c.GetClientIP()

		if !limiter.Allow(clientIdentifier) {
			c.JSON(http.StatusTooManyRequests, "rate limit exceeded")
			c.Abort()
			return
		}

		c.Next(ctx)
		return
	}
}
