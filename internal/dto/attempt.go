package dto

import "github.com/google/uuid"

type AttemptResponse struct {
	ID          uuid.UUID `json:"id"`
	QuestionID  uuid.UUID `json:"question_id"`
	UserID      uuid.UUID `json:"user_id"`
	Status      string    `json:"status"`
	NewBalance  float64   `json:"new_balance"`
	IsBuyInPaid bool      `json:"is_buy_in_paid"`
}
