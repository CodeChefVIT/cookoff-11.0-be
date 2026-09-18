package middlewares

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/auth"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/labstack/echo/v4"
)

// RateLimiterConfig holds per-user rate limiting settings.
type RateLimiterConfig struct {
	// Max requests allowed within the window.
	Max int
	// Sliding window duration.
	Window time.Duration
	// Skipper returns true for routes that skip rate limiting.
	Skipper func(echo.Context) bool
}

// RateLimiter returns an Echo middleware that enforces per-identity rate
func RateLimiter(cfg RateLimiterConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if cfg.Skipper != nil && cfg.Skipper(c) {
				return next(c)
			}

			identifier := extractIdentifier(c)
			key := fmt.Sprintf("rl:%s", identifier)

			ctx := c.Request().Context()
			rdb := utils.RedisClient

			// INCR atomically creates or bumps the counter.
			count, err := rdb.Incr(ctx, key).Result()
			if err != nil {
				// Redis down → fail-open so requests aren't blocked.
				logging.Errorf("rate limiter redis error: %v", err)
				return next(c)
			}

			// First request in window → set TTL.
			if count == 1 {
				rdb.Expire(ctx, key, cfg.Window)
			}

			if count > int64(cfg.Max) {
				ttl, _ := rdb.TTL(ctx, key).Result()
				c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())+1))
				return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
			}

			return next(c)
		}
	}
}

// extractIdentifier returns "u:<userID>" for authenticated requests or
// the client IP for anonymous ones. It reads the JWT directly from the
// cookie/header without going through VerifyJWTMiddleware, so it works
// as a global middleware that runs before route-level auth.
func extractIdentifier(c echo.Context) string {
	var tokenStr string
	if h := c.Request().Header.Get("Authorization"); h != "" {
		parts := strings.SplitN(h, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			tokenStr = parts[1]
		}
	}
	if tokenStr == "" {
		if cookie, err := c.Cookie(auth.AccessCookie); err == nil {
			tokenStr = cookie.Value
		}
	}
	if tokenStr != "" {
		if claims, err := auth.ParseToken(tokenStr, auth.AccessType); err == nil {
			return "u:" + claims.UserID
		}
	}
	return c.RealIP()
}
