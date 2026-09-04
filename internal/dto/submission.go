package dto

type SubmissionRequest struct {
	SourceCode string `json:"source_code" validate:"required"`
	LanguageID int    `json:"language_id" validate:"required"`
	QuestionID string `json:"question_id" validate:"required"`
}