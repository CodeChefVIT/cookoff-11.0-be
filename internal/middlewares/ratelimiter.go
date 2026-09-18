package middlewares

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/auth"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// Limit is a fixed-window budget: Max requests per Window.
type Limit struct {
	Max    int
	Window time.Duration
}

// RateLimiterConfig holds per-identity rate limiting settings.
type RateLimiterConfig struct {
	// Global applies to every request that is not skipped.
	Global Limit
	// Routes adds a stricter budget for specific "METHOD /path" routes (the
	// Echo route pattern, e.g. "POST /submit"), counted on top of Global.
	Routes map[string]Limit
	// Skipper returns true for requests that skip rate limiting.
	Skipper func(echo.Context) bool
}

// incrWindow bumps the counter and starts its window in one round trip, so a
// counter can never be left without an expiry (which would lock that identity
// out for good).
var incrWindow = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if redis.call("PTTL", KEYS[1]) < 0 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return {count, redis.call("PTTL", KEYS[1])}
`)

// RateLimiter returns an Echo middleware that enforces per-identity limits.
// Signed-in requests are keyed by user, everything else by client IP. It fails
// open when Redis is unavailable.
func RateLimiter(cfg RateLimiterConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if cfg.Skipper != nil && cfg.Skipper(c) {
				return next(c)
			}

			claims := accessClaims(c)
			// Admins are trusted and few; the admin panel's bulk actions send one
			// request per selected user in parallel and must not be cut off.
			if claims != nil && strings.EqualFold(claims.Role, "admin") {
				return next(c)
			}
			identifier := "ip:" + c.RealIP()
			if claims != nil {
				identifier = "u:" + claims.UserID
			}
			if retry, limited := take(c, "rl:"+identifier, cfg.Global); limited {
				return tooManyRequests(c, retry)
			}
			route := c.Request().Method + " " + c.Path()
			if limit, ok := cfg.Routes[route]; ok {
				if retry, limited := take(c, "rl:"+route+":"+identifier, limit); limited {
					return tooManyRequests(c, retry)
				}
			}
			return next(c)
		}
	}
}

// take counts one request against key and reports whether it is over limit,
// with how long until the window resets.
func take(c echo.Context, key string, limit Limit) (time.Duration, bool) {
	if limit.Max <= 0 || limit.Window <= 0 || utils.RedisClient == nil {
		return 0, false
	}
	res, err := incrWindow.Run(c.Request().Context(), utils.RedisClient, []string{key}, limit.Window.Milliseconds()).Int64Slice()
	if err != nil || len(res) != 2 {
		logging.Errorf("rate limiter redis error: %v", err)
		return 0, false
	}
	if res[0] <= int64(limit.Max) {
		return 0, false
	}
	return time.Duration(res[1]) * time.Millisecond, true
}

func tooManyRequests(c echo.Context, retry time.Duration) error {
	seconds := int(retry.Seconds())
	if retry%time.Second != 0 || seconds == 0 {
		seconds++
	}
	c.Response().Header().Set("Retry-After", strconv.Itoa(seconds))
	return c.JSON(http.StatusTooManyRequests, dto.NewCodedError("Too many requests, slow down", dto.CodeRateLimited))
}

// accessClaims reads and verifies the access token from the header or cookie
// without going through VerifyJWTMiddleware, so it works as a global
// middleware that runs before route-level auth. It returns nil for anonymous
// or invalid tokens.
func accessClaims(c echo.Context) *auth.Claims {
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
	if tokenStr == "" {
		return nil
	}
	claims, err := auth.ParseToken(tokenStr, auth.AccessType)
	if err != nil {
		return nil
	}
	return claims
}

// RateLimitSkipper exempts traffic that must never be throttled per IP: CORS
// preflights (they carry no credentials, so a whole lab behind one NAT would
// share a bucket), the OAuth hops and token refresh (same reason — they run
// before or without a valid access token), and the Judge0 callback and health
// probe.
func RateLimitSkipper(c echo.Context) bool {
	if c.Request().Method == http.MethodOptions {
		return true
	}
	p := c.Path()
	switch {
	case p == "/judge0callback", p == "/health", p == "/refreshToken", p == "/logout":
		return true
	case strings.HasPrefix(p, "/auth/"), strings.HasPrefix(p, "/api/v1/auth/"):
		return true
	}
	return false
}

// ClientIP prefers Cloudflare's client address, then a proxy-set
// X-Forwarded-For/X-Real-IP from a private (Nginx/Docker) hop, then the peer.
func ClientIP() echo.IPExtractor {
	fromXFF := echo.ExtractIPFromXFFHeader()
	return func(req *http.Request) string {
		if ip := strings.TrimSpace(req.Header.Get("CF-Connecting-IP")); ip != "" {
			return ip
		}
		return fromXFF(req)
	}
}
