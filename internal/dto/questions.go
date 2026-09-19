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
	BountyActive     bool      `json:"bounty_active"`
	ScratchBlocks    []string  `json:"scratch_blocks,omitempty"`
}

type QuestionRequest struct {
	Description      string   `json:"description" validate:"required"`
	Title            string   `json:"title" validate:"required"`
	Type             string   `json:"type" validate:"required"`
	InputFormat      []string `json:"input_format"`
	BuyIn            *float64 `json:"buy_in"`
	Reward           *float64 `json:"reward"`
	Points           int32    `json:"points" validate:"gte=0"`
	Round            int32    `json:"round" validate:"gte=0"`
	Constraints      []string `json:"constraints"`
	OutputFormat     []string `json:"output_format"`
	SampleTestInput  []string `json:"sample_test_input"`
	SampleTestOutput []string `json:"sample_test_output"`
	Explanation      []string `json:"explanation"`
	BountyActive     bool     `json:"bounty_active"`
	ScratchBlocks    []string `json:"scratch_blocks"`
	Solutions        [][]int  `json:"solutions"`
	SolutionPoints   []float64 `json:"solution_points"`
}

type TestcaseRequest struct {
	ExpectedOutput string  `json:"expected_output" validate:"required"`
	Memory         float64 `json:"memory" validate:"gte=0"`
	Input          string  `json:"input" validate:"required"`
	Hidden         bool    `json:"hidden"`
	Runtime        float64 `json:"runtime" validate:"gte=0"`
	QuestionID     string  `json:"question_id" validate:"required,uuid"`
}

type TestcaseUpdateRequest struct {
	ExpectedOutput *string  `json:"expected_output"`
	Memory         *float64 `json:"memory" validate:"omitempty,gte=0"`
	Input          *string  `json:"input"`
	Hidden         *bool    `json:"hidden"`
	Runtime        *float64 `json:"runtime" validate:"omitempty,gte=0"`
	QuestionID     *string  `json:"question_id" validate:"omitempty,uuid"`
}

type TestcaseResponse struct {
	ID             uuid.UUID `json:"id"`
	QuestionID     uuid.UUID `json:"question_id"`
	ExpectedOutput string    `json:"expected_output"`
	Input          string    `json:"input"`
	Memory         string    `json:"memory"`
	Runtime        string    `json:"runtime"`
	Hidden         bool      `json:"hidden"`
}

type VisualBlockResponse struct {
	ID      uuid.UUID `json:"id"`
	Content string    `json:"content"`
}
