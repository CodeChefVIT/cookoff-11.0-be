package controllers

import (
	"context"
	"net/http"
	"strconv"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
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
		ctx:=c.Request().Context()

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

		rows, e := q.ListDashboardQuestions(ctx, id)
		if e != nil {
			logging.Errorf("Dashboard error loading questions for user %s: %v", id, e)
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to load dashboard", nil))
		}

		d := dto.DashboardResponse{
			ID:             u.ID,
			Name:           u.Name,
			Email:          u.Email,
			RoundQualified: u.RoundQualified,
			Questions:      make([]dto.DashboardQuestion, len(rows)),
			AttemptTotals:  map[string]int{"available": 0, "bought": 0, "answered": 0},
			CurrentRound:   int(timer.GetCurrentRound(ctx)),
			RoundStatus: [3]dto.DashboardRoundStatus{
				{Round: 1},
				{Round: 2},
				{Round: 3},
			},
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
		maxScore := 0
		for i, r := range rows {
			d.Questions[i] = dto.DashboardQuestion{
				ID:            r.ID,
				Title:         r.Title,
				Points:        r.Points,
				Round:         r.Round,
				AttemptStatus: r.AttemptStatus,
			}

			if r.Round >= 1 && int(r.Round) <= len(d.RoundStatus) {
				idx := r.Round - 1
				if r.AttemptStatus == "answered" {
					d.RoundStatus[idx].QuestionsCompleted++
					d.RoundStatus[idx].Score += int(r.Points)
				} else if r.AttemptStatus == "bought" {
					d.RoundStatus[idx].QuestionsIncomplete++
				}
			}

			maxScore += int(r.Points)
			d.AttemptTotals[r.AttemptStatus]++
		}
		d.MaxScore = strconv.Itoa(maxScore)
		return c.JSON(http.StatusOK, dto.NewSuccessResponse("Dashboard retrieved", d))
	}
}
