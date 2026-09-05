package dto

import "github.com/google/uuid"

type SubmitVisualSolutionRequest struct {
	QuestionID uuid.UUID   `json:"question_id"`
	Blocks     []uuid.UUID `json:"blocks"`
}

type SubmitVisualSolutionResponse struct {
	PointsAwarded float64 `json:"points_awarded"`
}
