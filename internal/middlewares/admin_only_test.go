package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestAdminOnlyRejectsContestants(t *testing.T) {
	t.Parallel()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/question", nil)
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)
	c.Set(RoleKey, "participant")
	h := AdminOnly(func(echo.Context) error { return nil })
	err := h(c)
	if err == nil {
		t.Fatal("contestant was allowed through admin middleware")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %#v", err)
	}
}

func TestAdminOnlyAllowsAdmins(t *testing.T) {
	t.Parallel()
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodPost, "/question", nil), httptest.NewRecorder())
	c.Set(RoleKey, "ADMIN")
	called := false
	if err := AdminOnly(func(echo.Context) error { called = true; return nil })(c); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("admin handler was not called")
	}
}
