package dto

type TestcaseResult struct {
	ID          string  `json:"id"`
	Runtime     float64 `json:"runtime"`
	Memory      float64 `json:"memory"`
	Status      string  `json:"status"`
	Description string  `json:"description"`
}

type ResultResponse struct {
	ID             string           `json:"id"`
	QuestionID     string           `json:"question_id"`
	Passed         int              `json:"passed"`
	Failed         int              `json:"failed"`
	Runtime        float64          `json:"runtime"`
	Memory         float64          `json:"memory"`
	SubmissionTime string           `json:"submission_time"`
	Description    string           `json:"description"`
	Testcases      []TestcaseResult `json:"testcases"`
}
