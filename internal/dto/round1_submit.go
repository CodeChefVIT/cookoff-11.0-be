package dto

import "github.com/google/uuid"

type SubmitVisualSolutionRequest struct {
	QuestionID uuid.UUID   `json:"question_id"`
	Blocks     []uuid.UUID `json:"blocks"`
}

type SubmitVisualSolutionResponse struct {
	Status        string  `json:"status"`
	PointsAwarded float64 `json:"points_awarded"`
	Note          string  `json:"note,omitempty"`
}
