package dto

type SubmissionRequest struct {
	SourceCode string `json:"source_code" validate:"required,max=100000"`
	LanguageID int    `json:"language_id" validate:"required"`
	QuestionID string `json:"question_id" validate:"required,uuid"`
}

type CustomSubmissionRequest struct {
	SourceCode string `json:"source_code" validate:"required,max=100000"`
	LanguageID int    `json:"language_id" validate:"required"`
	Stdin      string `json:"stdin,omitempty" validate:"max=100000"`
}
