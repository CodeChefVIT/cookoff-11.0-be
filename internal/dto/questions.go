package dto

import "github.com/google/uuid"

// QuestionResponse contains only fields that may be shown to contest participants.
type QuestionResponse struct {
	ID               uuid.UUID `json:"id"`
	Description      string    `json:"description"`
	Title            string    `json:"title"`
	Type             string    `json:"type"`
	InputFormat      []string  `json:"input_format,omitempty"`
	BuyIn            string    `json:"buy_in,omitempty"`
	Reward           string    `json:"reward,omitempty"`
	Points           int32     `json:"points"`
	Round            int32     `json:"round"`
	Constraints      []string  `json:"constraints,omitempty"`
	OutputFormat     []string  `json:"output_format,omitempty"`
	SampleTestInput  []string  `json:"sample_test_input,omitempty"`
	SampleTestOutput []string  `json:"sample_test_output,omitempty"`
	Explanation      []string  `json:"explanation,omitempty"`
}

type VisualBlockResponse struct {
	ID      uuid.UUID `json:"id"`
	Content string    `json:"content"`
}
