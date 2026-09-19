package controllers

import (
	"context"
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type dashboardQueries interface {
	GetUserByID(context.Context, uuid.UUID) (sqlc.User, error)
	GetDashboardQuestions(context.Context, uuid.UUID) ([]sqlc.GetDashboardQuestionsRow, error)
}

// Dashboard is the player's profile plus their current round's questions
// with per-question attempt status. The portal also uses it as its session
// probe, so it stays at two cheap queries.
func Dashboard(q dashboardQueries) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, e := userID(c)
		if e != nil {
			return c.JSON(http.StatusUnauthorized, dto.NewCodedError("Unauthorized", dto.CodeUnauthorized))
		}
		ctx := c.Request().Context()

		u, e := q.GetUserByID(ctx, id)
		if e != nil {
			logging.Errorf("Dashboard error loading user %s: %v", id, e)
			return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to load dashboard", dto.CodeInternal))
		}

		questionsRows, e := q.GetDashboardQuestions(ctx, id)
		if e != nil {
			logging.Errorf("Dashboard error loading questions for user %s: %v", id, e)
			return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to load dashboard", dto.CodeInternal))
		}

		questions := make([]dto.DashboardQuestion, len(questionsRows))
		for i, qRow := range questionsRows {
			questions[i] = dto.DashboardQuestion{
				ID:            qRow.ID,
				Title:         qRow.Title,
				Points:        qRow.Points,
				Round:         qRow.Round,
				AttemptStatus: qRow.AttemptStatus,
			}
		}

		return c.JSON(http.StatusOK, dto.NewSuccessResponse("Dashboard retrieved", dto.DashboardResponse{
			ID:             u.ID,
			Name:           u.Name,
			Email:          u.Email,
			Balance:        numericText(u.Balance),
			Score:          numericText(u.Score),
			RoundQualified: u.RoundQualified,
			Questions:      questions,
		}))
	}
}
