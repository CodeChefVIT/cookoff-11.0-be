package middlewares

import (
	"net/http"
	"strings"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/auth"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	UserIDKey = "user_id"
	RoleKey   = "role"
)

func VerifyJWTMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cookie, err := c.Cookie(auth.AccessCookie)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
		}
		claims, err := auth.ParseToken(cookie.Value, auth.AccessType)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
		}
		c.Set(UserIDKey, claims.UserID)
		c.Set(RoleKey, strings.ToLower(claims.Role))
		return next(c)
	}
}

func BanCheckUser(queries *sqlc.Queries) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := c.Get(UserIDKey).(string)
			id, err := uuid.Parse(userID)
			if !ok || err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}
			user, err := queries.GetUserByID(c.Request().Context(), id)
			if err != nil || user.IsBanned {
				clearSession(c)
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}
			return next(c)
		}
	}
}

func AdminOnly(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		role, _ := c.Get(RoleKey).(string)
		if strings.ToLower(role) != "admin" {
			return echo.NewHTTPError(http.StatusForbidden, "admin access required")
		}
		return next(c)
	}
}

func clearSession(c echo.Context) {
	for _, cookie := range auth.ClearSessionCookies() {
		c.SetCookie(cookie)
	}
}

func JWTAuth(next echo.HandlerFunc) echo.HandlerFunc { return VerifyJWTMiddleware(next) }
