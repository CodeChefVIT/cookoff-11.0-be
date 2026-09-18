package dto

import "net/http"

type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ErrorResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Code    string      `json:"code,omitempty"`
	Errors  interface{} `json:"errors,omitempty"`
}

// Machine-readable error codes. Clients branch on these instead of matching
// the human-readable message, which is free to change.
const (
	CodeValidation      = "VALIDATION"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeForbidden       = "FORBIDDEN"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "ALREADY_EXISTS"
	CodeRateLimited     = "RATE_LIMITED"
	CodeInternal        = "INTERNAL"
	CodeRoundNotRunning = "ROUND_NOT_RUNNING"
	CodeNotQualified    = "NOT_QUALIFIED"
	CodeNotPurchased    = "NOT_PURCHASED"
	CodeInsufficient    = "INSUFFICIENT_BALANCE"
	CodeJudgeBusy       = "JUDGE_BUSY"
	CodeJudgeFailed     = "JUDGE_FAILED"
)

func NewSuccessResponse(message string, data interface{}) SuccessResponse {
	return SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
}

func NewErrorResponse(message string, errors interface{}) ErrorResponse {
	return ErrorResponse{
		Success: false,
		Message: message,
		Errors:  errors,
	}
}

// NewCodedError is an error body carrying a machine-readable code.
func NewCodedError(message, code string) ErrorResponse {
	return ErrorResponse{Success: false, Message: message, Code: code}
}

// CodeForStatus is the default code for a status when a handler gives none.
func CodeForStatus(status int) string {
	switch {
	case status == http.StatusBadRequest, status == http.StatusRequestEntityTooLarge:
		return CodeValidation
	case status == http.StatusUnauthorized:
		return CodeUnauthorized
	case status == http.StatusForbidden:
		return CodeForbidden
	case status == http.StatusNotFound, status == http.StatusMethodNotAllowed:
		return CodeNotFound
	case status == http.StatusConflict:
		return CodeConflict
	case status == http.StatusLocked:
		return CodeRoundNotRunning
	case status == http.StatusTooManyRequests:
		return CodeRateLimited
	case status >= http.StatusInternalServerError:
		return CodeInternal
	}
	return ""
}
