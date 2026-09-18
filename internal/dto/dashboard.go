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

type DashboardResponse struct {
	ID             uuid.UUID           `json:"id"`
	Name           string              `json:"name"`
	Email          string              `json:"email"`
	Balance        string              `json:"balance"`
	Score          string              `json:"score"`
	RoundQualified int32               `json:"round_qualified"`
	Questions      []DashboardQuestion `json:"questions"`
}
