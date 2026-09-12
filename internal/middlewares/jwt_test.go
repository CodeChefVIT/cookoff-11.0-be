package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/auth"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type mockUserGetter struct {
	users map[uuid.UUID]sqlc.User
}

func (m *mockUserGetter) GetUserByID(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	u, ok := m.users[id]
	if !ok {
		return sqlc.User{}, errors.New("user not found")
	}
	return u, nil
}

func TestVerifyJWTMiddleware(t *testing.T) {
	utils.Config.JWTSecret = "test_secret_for_jwt_auth_testing_key_123"
	utils.Config.AccessTokenTTL = 15 * time.Minute
	e := echo.New()

	handler := VerifyJWTMiddleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "authenticated")
	})

	// Missing cookie
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err == nil {
		t.Fatalf("expected error for missing access_token cookie")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %v", err)
	}

	// Valid token cookie
	userID := uuid.New().String()
	tokenStr, err := auth.NewAccessToken(auth.User{ID: userID, Email: "test@example.com", Role: "Admin"})
	if err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}

	reqValid := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	reqValid.AddCookie(&http.Cookie{Name: auth.AccessCookie, Value: tokenStr})
	recValid := httptest.NewRecorder()
	cValid := e.NewContext(reqValid, recValid)

	if err := handler(cValid); err != nil {
		t.Fatalf("unexpected error for valid JWT: %v", err)
	}
	if recValid.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recValid.Code)
	}
	if cValid.Get(UserIDKey) != userID {
		t.Fatalf("expected userID %s in context, got %v", userID, cValid.Get(UserIDKey))
	}
	if cValid.Get(RoleKey) != "admin" {
		t.Fatalf("expected role 'admin' in context, got %v", cValid.Get(RoleKey))
	}
}

func TestBanCheckUser(t *testing.T) {
	e := echo.New()
	mockGetter := &mockUserGetter{users: make(map[uuid.UUID]sqlc.User)}

	unbannedID := uuid.New()
	bannedID := uuid.New()

	mockGetter.users[unbannedID] = sqlc.User{ID: unbannedID, IsBanned: false}
	mockGetter.users[bannedID] = sqlc.User{ID: bannedID, IsBanned: true}

	middleware := BanCheckUser(mockGetter)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// Unbanned user
	reqUnbanned := httptest.NewRequest(http.MethodPost, "/submit", nil)
	recUnbanned := httptest.NewRecorder()
	cUnbanned := e.NewContext(reqUnbanned, recUnbanned)
	cUnbanned.Set(UserIDKey, unbannedID.String())

	if err := handler(cUnbanned); err != nil {
		t.Fatalf("unexpected error for unbanned user: %v", err)
	}
	if recUnbanned.Code != http.StatusOK {
		t.Fatalf("expected 200 for unbanned user, got %d", recUnbanned.Code)
	}

	// Banned user -> blocked from submitting code or accessing contest endpoints
	reqBanned := httptest.NewRequest(http.MethodPost, "/submit", nil)
	recBanned := httptest.NewRecorder()
	cBanned := e.NewContext(reqBanned, recBanned)
	cBanned.Set(UserIDKey, bannedID.String())

	err := handler(cBanned)
	if err == nil {
		t.Fatalf("expected error for banned user")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for banned user, got %v", err)
	}
}

func TestAdminOnly(t *testing.T) {
	e := echo.New()

	handler := AdminOnly(func(c echo.Context) error {
		return c.String(http.StatusOK, "admin ok")
	})

	// Admin role
	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	recAdmin := httptest.NewRecorder()
	cAdmin := e.NewContext(reqAdmin, recAdmin)
	cAdmin.Set(RoleKey, "admin")

	if err := handler(cAdmin); err != nil {
		t.Fatalf("unexpected error for admin: %v", err)
	}
	if recAdmin.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d", recAdmin.Code)
	}

	// Participant role
	reqUser := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	recUser := httptest.NewRecorder()
	cUser := e.NewContext(reqUser, recUser)
	cUser.Set(RoleKey, "participant")

	err := handler(cUser)
	if err == nil {
		t.Fatalf("expected error for participant accessing admin route")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for participant, got %v", err)
	}
}
