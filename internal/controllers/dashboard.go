package controllers

import (
	"context"
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/timer"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type dashboardQueries interface {
	GetUserByID(context.Context, uuid.UUID) (sqlc.User, error)
	GetDashboardRoundStats(context.Context, uuid.UUID) ([]sqlc.GetDashboardRoundStatsRow, error)
	GetDashboardQuestions(context.Context, uuid.UUID) ([]sqlc.GetDashboardQuestionsRow, error)
}

func Dashboard(q dashboardQueries) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, e := userID(c)
		if e != nil {
			return c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("unauthorized", nil))
		}
		ctx := c.Request().Context()

		u, e := q.GetUserByID(ctx, id)
		if e != nil {
			logging.Errorf("Dashboard error loading user %s: %v", id, e)
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to load dashboard", nil))
		}

		currentTime, err := timer.GetTime(ctx)
		if err != nil {
			logging.Errorf("Dashboard error getting time %s: %v", id, err)
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to load dashboard", nil))
		}

		statsRows, e := q.GetDashboardRoundStats(ctx, id)
		if e != nil {
			logging.Errorf("Dashboard error loading stats for user %s: %v", id, e)
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to load dashboard", nil))
		}

		questionsRows, e := q.GetDashboardQuestions(ctx, id)
		if e != nil {
			logging.Errorf("Dashboard error loading questions for user %s: %v", id, e)
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to load dashboard", nil))
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

		d := dto.DashboardResponse{
			ID:             u.ID,
			Name:           u.Name,
			Email:          u.Email,
			RoundQualified: u.RoundQualified,
			AttemptTotals:  map[string]int{"available": 0, "bought": 0, "answered": 0},
			CurrentRound:   int(timer.GetCurrentRound(ctx)),
			RoundStatus: [3]dto.DashboardRoundStatus{
				{Round: 1},
				{Round: 2},
				{Round: 3},
			},
			Questions: questions,
		}

		for i := 0; i < 3; i++ {
			if int(currentTime.Round-1) == i && i <= int(u.RoundQualified) {
				d.RoundStatus[i].Status = "open"
			} else if int(currentTime.Round-1) < i || int(u.RoundQualified) < i {
				d.RoundStatus[i].Status = "locked"
			} else {
				d.RoundStatus[i].Status = "closed"
			}
		}

		d.Balance = numericText(u.Balance)
		d.Score = numericText(u.Score)
		for _, row := range statsRows {
			if row.Round >= 1 && int(row.Round) <= len(d.RoundStatus) {
				idx := row.Round - 1
				d.RoundStatus[idx].QuestionsCompleted = int(row.QuestionsCompleted)
				d.RoundStatus[idx].QuestionsIncomplete = int(row.QuestionsIncomplete)
				d.RoundStatus[idx].Score = int(row.RoundScore)
			}
		}
		return c.JSON(http.StatusOK, dto.NewSuccessResponse("Dashboard retrieved", d))
	}
}
