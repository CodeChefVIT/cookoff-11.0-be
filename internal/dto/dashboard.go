package dto

import (
	"github.com/google/uuid"
)

type DashboardQuestion struct {
	ID            uuid.UUID `json:"id"`
	Title         string    `json:"title"`
	Points        int32     `json:"points"`
	Round         int32     `json:"round"`
	AttemptStatus string    `json:"attempt_status"`
}

type DashboardRoundStatus struct {
	Round               int    `json:"round"`
	Status              string `json:"status"`
	QuestionsCompleted  int    `json:"questions_completed"`
	QuestionsIncomplete int    `json:"questions_incomplete"`
	Score               int    `json:"score"`
}

type DashboardResponse struct {
	ID             uuid.UUID               `json:"id"`
	Name           string                  `json:"name"`
	Email          string                  `json:"email"`
	Balance        string                  `json:"balance"`
	Score          string                  `json:"score"`
	MaxScore       string                  `json:"max_score"`
	RoundQualified int32                   `json:"round_qualified"`
	Questions      []DashboardQuestion     `json:"questions"`
	AttemptTotals  map[string]int          `json:"attempt_totals"`
	CurrentRound   int                     `json:"current_round"`
	RoundStatus    [3]DashboardRoundStatus `json:"round_status"`
}
