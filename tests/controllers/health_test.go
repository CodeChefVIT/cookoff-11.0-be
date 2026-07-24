package controllers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodeChefVIT/go-backend-template/internal/controllers"
	"github.com/CodeChefVIT/go-backend-template/internal/dto"
	"github.com/labstack/echo/v4"
)

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		expectedStatus int
		expectedPG     string
		expectedRedis  string
	}{
		{
			name:           "both_down_uninitialized",
			expectedStatus: http.StatusServiceUnavailable,
			expectedPG:     "DOWN",
			expectedRedis:  "DOWN",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := controllers.HealthCheck(c)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			var resp dto.SuccessResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response body: %v", err)
			}

			if resp.Message != "Health check completed" {
				t.Errorf("expected message 'Health check completed', got '%s'", resp.Message)
			}

			data, ok := resp.Data.(map[string]interface{})
			if !ok {
				t.Fatalf("response data is not a map")
			}

			if data["postgres"] != tt.expectedPG {
				t.Errorf("expected postgres=%s, got %v", tt.expectedPG, data["postgres"])
			}
			if data["redis"] != tt.expectedRedis {
				t.Errorf("expected redis=%s, got %v", tt.expectedRedis, data["redis"])
			}
		})
	}
}

// TODO: Add integration tests with testcontainers for:
// - postgres_up_redis_down
// - postgres_down_redis_up
// - both_up
// This requires mockable DB/Redis (refactor global state)
