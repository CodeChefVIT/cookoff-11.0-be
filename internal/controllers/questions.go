package controllers

import (
	"context"
	"errors"
	"strconv"
	"strings"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
)

type questionQueries interface {
	ListQuestionsForUser(context.Context, uuid.UUID) ([]sqlc.ListQuestionsForUserRow, error)
	GetQuestionForUser(context.Context, sqlc.GetQuestionForUserParams) (sqlc.GetQuestionForUserRow, error)
	ListVisualBlocksByQuestionID(context.Context, uuid.UUID) ([]sqlc.VisualBlock, error)
	CreateQuestion(context.Context, sqlc.CreateQuestionParams) (sqlc.Question, error)
	UpdateQuestion(context.Context, sqlc.UpdateQuestionParams) (sqlc.Question, error)
	DeleteQuestion(context.Context, uuid.UUID) (uuid.UUID, error)
	SetQuestionBountyActive(context.Context, sqlc.SetQuestionBountyActiveParams) (sqlc.Question, error)
}

type QuestionController struct{ queries questionQueries }

func NewQuestionController(q questionQueries) *QuestionController {
	return &QuestionController{queries: q}
}
func userID(c echo.Context) (uuid.UUID, error) {
	raw, ok := c.Get(middlewares.UserIDKey).(string)
	if !ok {
		return uuid.Nil, errors.New("unauthorized")
	}
	return uuid.Parse(raw)
}
func parseQuestionID(c echo.Context) (uuid.UUID, error) { return uuid.Parse(c.Param("id")) }
func questionError(c echo.Context, s int, m string) error {
	return c.JSON(s, dto.NewErrorResponse(m, nil))
}

func (qc *QuestionController) ListByRound(c echo.Context) error {
	id, e := userID(c)
	if e != nil {
		return questionError(c, 401, "unauthorized")
	}
	rows, e := qc.queries.ListQuestionsForUser(c.Request().Context(), id)
	if e != nil {
		return questionError(c, 500, "failed to load questions")
	}
	out := make([]dto.QuestionResponse, len(rows))
	for i, q := range rows {
		out[i] = questionFromRow(q.ID, q.Description, q.Title, q.QType, q.InputFormat, q.BuyIn, q.Reward, q.Points, q.Round, q.Constraints, q.OutputFormat, q.SampleTestInput, q.SampleTestOutput, q.Explanation, q.BountyActive)
	}
	return c.JSON(200, dto.NewSuccessResponse("Questions retrieved", out))
}
func (qc *QuestionController) GetByID(c echo.Context) error {
	qid, e := parseQuestionID(c)
	if e != nil {
		return questionError(c, 400, "invalid question ID")
	}
	uid, e := userID(c)
	if e != nil {
		return questionError(c, 401, "unauthorized")
	}
	q, e := qc.queries.GetQuestionForUser(c.Request().Context(), sqlc.GetQuestionForUserParams{ID: qid, ID_2: uid})
	if errors.Is(e, pgx.ErrNoRows) {
		return questionError(c, 404, "question not found")
	}
	if e != nil {
		return questionError(c, 500, "failed to load question")
	}
	return c.JSON(200, dto.NewSuccessResponse("Question retrieved", questionFromRow(q.ID, q.Description, q.Title, q.QType, q.InputFormat, q.BuyIn, q.Reward, q.Points, q.Round, q.Constraints, q.OutputFormat, q.SampleTestInput, q.SampleTestOutput, q.Explanation, q.BountyActive)))
}
func (qc *QuestionController) ListBlocks(c echo.Context) error {
	qid, e := parseQuestionID(c)
	if e != nil {
		return questionError(c, 400, "invalid question ID")
	}
	uid, e := userID(c)
	if e != nil {
		return questionError(c, 401, "unauthorized")
	}
	q, e := qc.queries.GetQuestionForUser(c.Request().Context(), sqlc.GetQuestionForUserParams{ID: qid, ID_2: uid})
	if e != nil || q.Round != 1 || !strings.EqualFold(q.QType, "visual") {
		return questionError(c, 404, "Round 1 visual question not found")
	}
	blocks, e := qc.queries.ListVisualBlocksByQuestionID(c.Request().Context(), qid)
	if e != nil {
		return questionError(c, 500, "failed to load visual blocks")
	}
	out := make([]dto.VisualBlockResponse, len(blocks))
	for i, b := range blocks {
		out[i] = dto.VisualBlockResponse{ID: b.ID, Content: b.Content}
	}
	return c.JSON(200, dto.NewSuccessResponse("Visual blocks retrieved", out))
}
func (qc *QuestionController) Create(c echo.Context) error {
	var r dto.QuestionRequest
	if e := c.Bind(&r); e != nil {
		return questionError(c, 400, "invalid request body")
	}
	if e := c.Validate(&r); e != nil {
		return questionError(c, 400, "validation failed")
	}
	q, e := qc.queries.CreateQuestion(c.Request().Context(), questionParams(uuid.New(), r))
	if e != nil {
		return questionError(c, 500, "failed to create question")
	}
	return c.JSON(201, dto.NewSuccessResponse("Question created", questionFromModel(q)))
}
func (qc *QuestionController) Update(c echo.Context) error {
	id, e := parseQuestionID(c)
	if e != nil {
		return questionError(c, 400, "invalid question ID")
	}
	var r dto.QuestionRequest
	if e = c.Bind(&r); e != nil {
		return questionError(c, 400, "invalid request body")
	}
	if e = c.Validate(&r); e != nil {
		return questionError(c, 400, "validation failed")
	}
	q, e := qc.queries.UpdateQuestion(c.Request().Context(), sqlc.UpdateQuestionParams(questionParams(id, r)))
	if errors.Is(e, pgx.ErrNoRows) {
		return questionError(c, 404, "question not found")
	}
	if e != nil {
		return questionError(c, 500, "failed to update question")
	}
	return c.JSON(200, dto.NewSuccessResponse("Question updated", questionFromModel(q)))
}
func (qc *QuestionController) Delete(c echo.Context) error {
	id, e := parseQuestionID(c)
	if e != nil {
		return questionError(c, 400, "invalid question ID")
	}
	_, e = qc.queries.DeleteQuestion(c.Request().Context(), id)
	if errors.Is(e, pgx.ErrNoRows) {
		return questionError(c, 404, "question not found")
	}
	if e != nil {
		return questionError(c, 500, "failed to delete question")
	}
	return c.JSON(200, dto.NewSuccessResponse("Question deleted", nil))
}
func (qc *QuestionController) SetBounty(active bool) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, e := parseQuestionID(c)
		if e != nil {
			return questionError(c, 400, "invalid question ID")
		}
		q, e := qc.queries.SetQuestionBountyActive(c.Request().Context(), sqlc.SetQuestionBountyActiveParams{ID: id, BountyActive: active})
		if errors.Is(e, pgx.ErrNoRows) {
			return questionError(c, 404, "question not found")
		}
		if e != nil {
			return questionError(c, 500, "failed to update bounty")
		}
		return c.JSON(200, dto.NewSuccessResponse("Bounty updated", questionFromModel(q)))
	}
}
func questionParams(id uuid.UUID, r dto.QuestionRequest) sqlc.CreateQuestionParams {
	p := sqlc.CreateQuestionParams{ID: id, Description: r.Description, Title: r.Title, QType: r.Type, InputFormat: r.InputFormat, Points: r.Points, Round: r.Round, Constraints: r.Constraints, OutputFormat: r.OutputFormat, SampleTestInput: r.SampleTestInput, SampleTestOutput: r.SampleTestOutput, Explanation: r.Explanation, BountyActive: r.BountyActive}
	if r.BuyIn != nil {
		p.BuyIn, _ = utils.Float64ToNumeric(*r.BuyIn)
	}
	if r.Reward != nil {
		p.Reward, _ = utils.Float64ToNumeric(*r.Reward)
	}
	return p
}
func questionFromModel(q sqlc.Question) dto.QuestionResponse {
	return questionFromRow(q.ID, q.Description, q.Title, q.QType, q.InputFormat, q.BuyIn, q.Reward, q.Points, q.Round, q.Constraints, q.OutputFormat, q.SampleTestInput, q.SampleTestOutput, q.Explanation, q.BountyActive)
}
func questionFromRow(id uuid.UUID, d, t, typ string, in []string, buy, reward interface{}, pts, rnd int32, cons, out, sin, sout, exp []string, active bool) dto.QuestionResponse {
	return dto.QuestionResponse{ID: id, Description: d, Title: t, Type: typ, InputFormat: in, BuyIn: textValue(buy), Reward: textValue(reward), Points: pts, Round: rnd, Constraints: cons, OutputFormat: out, SampleTestInput: sin, SampleTestOutput: sout, Explanation: exp, BountyActive: active}
}
func textValue(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case pgtype.Numeric:
		f, e := utils.NumericToFloat64(x)
		if e == nil {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
	}
	return ""
}
