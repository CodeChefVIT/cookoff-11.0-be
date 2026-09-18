package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func newLimitedEcho(t *testing.T, cfg RateLimiterConfig) (*echo.Echo, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	prev := utils.RedisClient
	utils.RedisClient = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { utils.RedisClient = prev })

	e := echo.New()
	e.Use(RateLimiter(cfg))
	ok := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	e.GET("/getTime", ok)
	e.POST("/submit", ok)
	e.GET("/auth/google", ok)
	e.OPTIONS("/submit", ok)
	return e, mr
}

func do(e *echo.Echo, method, path, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestRateLimiterGlobalBudget(t *testing.T) {
	e, _ := newLimitedEcho(t, RateLimiterConfig{Global: Limit{Max: 3, Window: time.Second}, Skipper: RateLimitSkipper})
	for i := 0; i < 3; i++ {
		if rec := do(e, http.MethodGet, "/getTime", "10.0.0.1"); rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d", i, rec.Code)
		}
	}
	rec := do(e, http.MethodGet, "/getTime", "10.0.0.1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over budget: got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	// A different client has its own bucket.
	if rec := do(e, http.MethodGet, "/getTime", "10.0.0.2"); rec.Code != http.StatusOK {
		t.Fatalf("other client: got %d", rec.Code)
	}
}

func TestRateLimiterWindowAlwaysExpires(t *testing.T) {
	e, mr := newLimitedEcho(t, RateLimiterConfig{Global: Limit{Max: 1, Window: time.Second}})
	do(e, http.MethodGet, "/getTime", "10.0.0.1")
	// Simulate a counter that lost its expiry (the old INCR-then-EXPIRE race).
	if err := mr.Set("rl:ip:10.0.0.1", "1"); err != nil {
		t.Fatal(err)
	}
	do(e, http.MethodGet, "/getTime", "10.0.0.1")
	if ttl := mr.TTL("rl:ip:10.0.0.1"); ttl <= 0 {
		t.Fatalf("counter left without expiry: ttl=%v", ttl)
	}
	mr.FastForward(2 * time.Second)
	if rec := do(e, http.MethodGet, "/getTime", "10.0.0.1"); rec.Code != http.StatusOK {
		t.Fatalf("after window: got %d", rec.Code)
	}
}

func TestRateLimiterRouteBudget(t *testing.T) {
	e, _ := newLimitedEcho(t, RateLimiterConfig{
		Global: Limit{Max: 100, Window: time.Second},
		Routes: map[string]Limit{"POST /submit": {Max: 1, Window: 5 * time.Second}},
	})
	if rec := do(e, http.MethodPost, "/submit", "10.0.0.1"); rec.Code != http.StatusOK {
		t.Fatalf("first submit: got %d", rec.Code)
	}
	if rec := do(e, http.MethodPost, "/submit", "10.0.0.1"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second submit: got %d", rec.Code)
	}
	// The route budget does not spill onto other routes.
	if rec := do(e, http.MethodGet, "/getTime", "10.0.0.1"); rec.Code != http.StatusOK {
		t.Fatalf("other route: got %d", rec.Code)
	}
}

func TestRateLimiterSkipsPreflightAndAuth(t *testing.T) {
	e, _ := newLimitedEcho(t, RateLimiterConfig{Global: Limit{Max: 1, Window: time.Minute}, Skipper: RateLimitSkipper})
	for i := 0; i < 5; i++ {
		if rec := do(e, http.MethodOptions, "/submit", "10.0.0.1"); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("preflight %d throttled", i)
		}
		if rec := do(e, http.MethodGet, "/auth/google", "10.0.0.1"); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("oauth %d throttled", i)
		}
	}
}

func TestRateLimiterFailsOpenWithoutRedis(t *testing.T) {
	e, mr := newLimitedEcho(t, RateLimiterConfig{Global: Limit{Max: 1, Window: time.Second}})
	mr.Close()
	for i := 0; i < 3; i++ {
		if rec := do(e, http.MethodGet, "/getTime", "10.0.0.1"); rec.Code != http.StatusOK {
			t.Fatalf("request %d with redis down: got %d", i, rec.Code)
		}
	}
}

func TestClientIPPrefersCloudflare(t *testing.T) {
	extract := ClientIP()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 172.68.1.1")
	req.Header.Set("CF-Connecting-IP", "203.0.113.9")
	if got := extract(req); got != "203.0.113.9" {
		t.Fatalf("got %q", got)
	}
}
