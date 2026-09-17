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
	// Correct reports whether the chain matched a stored solution. It is not
	// the same as PointsAwarded > 0: a question that is already answered scores
	// zero on a resubmission even when the chain is right, and deriving
	// correctness from the points alone told the player their correct answer
	// was wrong.
	Correct bool `json:"correct"`
	// AlreadyAnswered reports that the attempt was settled before this
	// submission, so no further points or coins were paid out.
	AlreadyAnswered bool `json:"already_answered"`
}
