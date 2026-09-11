package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
)

func TestAccessTokenCannotBeUsedAsRefreshToken(t *testing.T) {
	utils.Config.JWTSecret = "test-secret"
	utils.Config.AccessTokenTTL = time.Minute
	utils.Config.RefreshTokenTTL = time.Hour
	token, err := NewAccessToken(User{ID: "user-id", Email: "user@example.com", Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseToken(token, RefreshType); err == nil {
		t.Fatal("access token was accepted as refresh token")
	}
}

func TestStateValidation(t *testing.T) {
	state, cookie, err := NewState("admin")
	if err != nil {
		t.Fatal(err)
	}
	portal, ok := ValidateState(cookie, state)
	if !ok || portal != "admin" {
		t.Fatal("valid state was rejected")
	}
	if _, ok := ValidateState(cookie, "other"); ok {
		t.Fatal("invalid state was accepted")
	}
}

func TestSessionCookiesAreSecureAndHTTPOnly(t *testing.T) {
	utils.Config.JWTSecret = "test-secret"
	utils.Config.AccessTokenTTL = time.Minute
	utils.Config.RefreshTokenTTL = time.Hour
	utils.Config.CookieSecure = true
	cookies, err := SessionCookies(User{ID: "user-id", Email: "user@example.com", Role: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cookies) != 2 {
		t.Fatal("expected access and refresh cookies")
	}
	for _, cookie := range cookies {
		if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
			t.Fatal("session cookie is missing required security attributes")
		}
	}
}
