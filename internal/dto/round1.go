package dto

import "github.com/google/uuid"

type SubmitVisualSolutionRequest struct {
	QuestionID uuid.UUID   `json:"question_id"`
	Blocks     []uuid.UUID `json:"blocks"`
}
