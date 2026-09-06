package controllers

import (
	"errors"
	"net/http"
	"strconv"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

type QuestionController struct {
	queries sqlc.Querier
}

func NewQuestionController(queries sqlc.Querier) *QuestionController {
	return &QuestionController{queries: queries}
}

func (qc *QuestionController) ListByRound(c echo.Context) error {
	round, err := strconv.ParseInt(c.QueryParam("round"), 10, 32)
	if err != nil || round < 1 {
		return questionError(c, http.StatusBadRequest, "round must be a positive integer")
	}

	questions, err := qc.queries.ListQuestionsByRound(c.Request().Context(), int32(round))
	if err != nil {
		return questionError(c, http.StatusInternalServerError, "failed to load questions")
	}

	response := make([]dto.QuestionResponse, len(questions))
	for i, question := range questions {
		response[i] = questionListResponse(question)
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Questions retrieved", response))
}

func (qc *QuestionController) GetByID(c echo.Context) error {
	questionID, err := parseQuestionID(c)
	if err != nil {
		return questionError(c, http.StatusBadRequest, "invalid question ID")
	}

	question, err := qc.queries.GetQuestionByID(c.Request().Context(), questionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return questionError(c, http.StatusNotFound, "question not found")
	}
	if err != nil {
		return questionError(c, http.StatusInternalServerError, "failed to load question")
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Question retrieved", questionResponse(question)))
}

func (qc *QuestionController) ListBlocks(c echo.Context) error {
	questionID, err := parseQuestionID(c)
	if err != nil {
		return questionError(c, http.StatusBadRequest, "invalid question ID")
	}

	if _, err := qc.queries.GetRoundOneVisualQuestion(c.Request().Context(), questionID); errors.Is(err, pgx.ErrNoRows) {
		return questionError(c, http.StatusNotFound, "Round 1 visual question not found")
	} else if err != nil {
		return questionError(c, http.StatusInternalServerError, "failed to verify question")
	}

	blocks, err := qc.queries.ListVisualBlocksByQuestionID(c.Request().Context(), questionID)
	if err != nil {
		return questionError(c, http.StatusInternalServerError, "failed to load visual blocks")
	}

	response := make([]dto.VisualBlockResponse, len(blocks))
	for i, block := range blocks {
		response[i] = dto.VisualBlockResponse{ID: block.ID, Content: block.Content}
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Visual blocks retrieved", response))
}

func parseQuestionID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

func questionError(c echo.Context, status int, message string) error {
	return c.JSON(status, dto.NewErrorResponse(message, nil))
}

func questionResponse(question sqlc.GetQuestionByIDRow) dto.QuestionResponse {
	return dto.QuestionResponse{
		ID:               question.ID,
		Description:      question.Description,
		Title:            question.Title,
		Type:             question.QType,
		InputFormat:      question.InputFormat,
		BuyIn:            textValue(question.BuyIn),
		Reward:           textValue(question.Reward),
		Points:           question.Points,
		Round:            question.Round,
		Constraints:      question.Constraints,
		OutputFormat:     question.OutputFormat,
		SampleTestInput:  question.SampleTestInput,
		SampleTestOutput: question.SampleTestOutput,
		Explanation:      question.Explanation,
	}
}

func questionListResponse(question sqlc.ListQuestionsByRoundRow) dto.QuestionResponse {
	return dto.QuestionResponse{
		ID:               question.ID,
		Description:      question.Description,
		Title:            question.Title,
		Type:             question.QType,
		InputFormat:      question.InputFormat,
		BuyIn:            textValue(question.BuyIn),
		Reward:           textValue(question.Reward),
		Points:           question.Points,
		Round:            question.Round,
		Constraints:      question.Constraints,
		OutputFormat:     question.OutputFormat,
		SampleTestInput:  question.SampleTestInput,
		SampleTestOutput: question.SampleTestOutput,
		Explanation:      question.Explanation,
	}
}

func textValue(value interface{}) string {
	switch value := value.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return ""
	}
}
