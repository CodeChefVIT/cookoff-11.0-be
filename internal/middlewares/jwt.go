package middlewares

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/auth"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

const (
	UserIDKey   = "user_id"
	RoleKey     = "role"
	AuthUserKey = "auth_user"
)

// AuthUser is the slice of the users row that authorization needs. It is
// loaded once per request by BanCheckUser (and cached briefly in Redis), so
// handlers never re-query it and roles/bans come from the database rather
// than from a JWT claim that outlives a change.
type AuthUser struct {
	ID             uuid.UUID `json:"id"`
	Role           string    `json:"role"`
	RoundQualified int32     `json:"round_qualified"`
	IsBanned       bool      `json:"is_banned"`
}

type userLoader interface {
	GetUserByID(context.Context, uuid.UUID) (sqlc.User, error)
}

func VerifyJWTMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var tokenStr string
		if authHeader := c.Request().Header.Get("Authorization"); authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				tokenStr = parts[1]
			}
		}
		if tokenStr == "" {
			cookie, err := c.Cookie(auth.AccessCookie)
			if err == nil {
				tokenStr = cookie.Value
			}
		}
		if tokenStr == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
		}
		claims, err := auth.ParseToken(tokenStr, auth.AccessType)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
		}
		c.Set(UserIDKey, claims.UserID)
		c.Set(RoleKey, strings.ToLower(claims.Role))
		return next(c)
	}
}

// BanCheckUser loads the signed-in user, rejects banned or deleted accounts,
// and stores the user for AdminOnly and the handlers (see CurrentUser).
func BanCheckUser(queries userLoader) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := c.Get(UserIDKey).(string)
			id, err := uuid.Parse(userID)
			if !ok || err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}
			user, err := loadAuthUser(c.Request().Context(), queries, id)
			if errors.Is(err, pgx.ErrNoRows) {
				clearSession(c)
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}
			if err != nil {
				// A database hiccup is not a reason to sign the player out.
				return echo.NewHTTPError(http.StatusServiceUnavailable, "temporarily unavailable")
			}
			if user.IsBanned {
				clearSession(c)
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}
			c.Set(AuthUserKey, user)
			c.Set(RoleKey, strings.ToLower(user.Role))
			return next(c)
		}
	}
}

// CurrentUser returns the user loaded by BanCheckUser.
func CurrentUser(c echo.Context) (AuthUser, bool) {
	user, ok := c.Get(AuthUserKey).(AuthUser)
	return user, ok
}

func loadAuthUser(ctx context.Context, queries userLoader, id uuid.UUID) (AuthUser, error) {
	key := utils.AuthUserKey(id.String())
	if utils.RedisClient != nil {
		if raw, err := utils.RedisClient.Get(ctx, key).Bytes(); err == nil {
			var cached AuthUser
			if json.Unmarshal(raw, &cached) == nil {
				return cached, nil
			}
		}
	}

	row, err := queries.GetUserByID(ctx, id)
	if err != nil {
		return AuthUser{}, err
	}
	user := AuthUser{ID: row.ID, Role: row.Role, RoundQualified: row.RoundQualified, IsBanned: row.IsBanned}

	if utils.RedisClient != nil {
		if raw, err := json.Marshal(user); err == nil {
			if setErr := utils.RedisClient.Set(ctx, key, raw, utils.AuthUserTTL).Err(); setErr != nil {
				logging.Warnf("cache auth user %s: %v", id, setErr)
			}
		}
	}
	return user, nil
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
