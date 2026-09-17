package controllers

import (
	"context"
	"net/http"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/timer"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type dashboardQueries interface {
	GetUserByID(context.Context, uuid.UUID) (sqlc.User, error)
	ListDashboardQuestions(context.Context, uuid.UUID) ([]sqlc.ListDashboardQuestionsRow, error)
}

func Dashboard(q dashboardQueries) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, e := userID(c)
		if e != nil {
			return c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("unauthorized", nil))
		}
		u, e := q.GetUserByID(c.Request().Context(), id)
		if e != nil {
			logging.Errorf("Dashboard error loading user %s: %v", id, e)
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("failed to load dashboard", nil))
		}

		ctx := c.Request().Context()
		currentTime, err := timer.GetTime(ctx);
		if err!=nil{
			logging.Errorf("Dashboard error getting time %s: %v", id, e)
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("failed to load dashboard", nil))
		}
		
		rows, e := q.ListDashboardQuestions(c.Request().Context(), id)
		if e != nil {
			logging.Errorf("Dashboard error loading questions for user %s: %v", id, e)
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("failed to load dashboard", nil))
		}
		d := dto.DashboardResponse{
			ID: u.ID,
			Name: u.Name,
			Email: u.Email,
			RoundQualified: u.RoundQualified,
			Questions: make([]dto.DashboardQuestion,len(rows)),
			AttemptTotals: map[string]int{"available": 0, "bought": 0, "answered": 0},
			CurrentRound: int(timer.GetCurrentRound(ctx)),
			RoundStatus: [3]dto.DashboardRoundStatus{
				{Round:1, Status:"inactive"},
				{Round:2, Status:"inactive"},
				{Round:3, Status:"inactive"},
			},
		}
		d.RoundStatus[currentTime.Round-1].Status="active"
		d.Balance = numericText(u.Balance)
		d.Score = numericText(u.Score)
		for i, r := range rows {
			d.Questions[i] = dto.DashboardQuestion{ID: r.ID, Title: r.Title, Points: r.Points, Round: r.Round, AttemptStatus: r.AttemptStatus}
			if r.AttemptStatus=="answered" {
				d.RoundStatus[r.Round-1].QuestionsCompleted++
				d.RoundStatus[r.Round-1].Score+=int(r.Points)
			}else if r.AttemptStatus=="bought"{
				d.RoundStatus[r.Round-1].QuestionIncomplete++
			}

			d.AttemptTotals[r.AttemptStatus]++
		}
		return c.JSON(http.StatusOK, dto.NewSuccessResponse("Dashboard retrieved", d))
	}
}
