package router

import (
	"testing"

	"github.com/labstack/echo/v4"
)

func TestManagementRoutesAreRegistered(t *testing.T) {
	t.Parallel()
	e := echo.New()
	RegisterRoutes(e)
	want := map[string]bool{
		"POST /question":                       false,
		"PUT /question/:id":                    false,
		"DELETE /question/:id":                 false,
		"POST /question/:id/bounty/activate":   false,
		"POST /question/:id/bounty/deactivate": false,
		"POST /testcase":                       false,
		"PUT /testcase/:id":                    false,
		"DELETE /testcase/:id":                 false,
		"GET /question/:id/testcases/public":   false,
		"GET /question/:id/testcases":          false,
		"GET /dashboard":                       false,
	}
	for _, route := range e.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("missing route %s", route)
		}
	}
}
