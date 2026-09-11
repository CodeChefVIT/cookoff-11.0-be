package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessCookie  = "access_token"
	RefreshCookie = "refresh_token"
	StateCookie   = "oauth_state"
	AccessType    = "access"
	RefreshType   = "refresh"
)

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

type User struct {
	ID    string
	Email string
	Role  string
}

func NewAccessToken(user User) (string, error) {
	return newToken(user, AccessType, utils.Config.AccessTokenTTL)
}
func NewRefreshToken(user User) (string, error) {
	return newToken(user, RefreshType, utils.Config.RefreshTokenTTL)
}

func newToken(user User, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   strings.ToLower(user.Role),
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(utils.Config.JWTSecret))
}

func ParseToken(raw, tokenType string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(utils.Config.JWTSecret), nil
	})
	if err != nil || !token.Valid || claims.Type != tokenType || claims.UserID == "" {
		return nil, fmt.Errorf("invalid %s token", tokenType)
	}
	return claims, nil
}

func SessionCookies(user User) ([]*http.Cookie, error) {
	access, err := NewAccessToken(user)
	if err != nil {
		return nil, err
	}
	refresh, err := NewRefreshToken(user)
	if err != nil {
		return nil, err
	}
	return []*http.Cookie{newCookie(AccessCookie, access, utils.Config.AccessTokenTTL), newCookie(RefreshCookie, refresh, utils.Config.RefreshTokenTTL)}, nil
}

func ClearSessionCookies() []*http.Cookie {
	return []*http.Cookie{newCookie(AccessCookie, "", -time.Hour), newCookie(RefreshCookie, "", -time.Hour)}
}

func NewState() (string, *http.Cookie, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", nil, err
	}
	state := base64.RawURLEncoding.EncodeToString(bytes)
	return state, newCookie(StateCookie, state, 10*time.Minute), nil
}

func ValidateState(cookie *http.Cookie, state string) bool {
	return cookie != nil && state != "" && cookie.Value == state
}

func ClearStateCookie() *http.Cookie { return newCookie(StateCookie, "", -time.Hour) }

func newCookie(name, value string, ttl time.Duration) *http.Cookie {
	cookie := &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: utils.Config.CookieSecure, SameSite: http.SameSiteLaxMode} // #nosec G124 -- local HTTP development disables Secure; production config enables it.
	// Browsers treat localhost as a special host and may drop cookies that
	// explicitly declare Domain=localhost. Leave the domain unset locally;
	// production deployments can still share cookies across subdomains.
	if domain := strings.TrimSpace(utils.Config.CookieDomain); domain != "" && domain != "localhost" {
		cookie.Domain = domain
	}
	if ttl <= 0 {
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
		return cookie
	}
	cookie.MaxAge = int(ttl.Seconds())
	cookie.Expires = time.Now().Add(ttl)
	return cookie
}
