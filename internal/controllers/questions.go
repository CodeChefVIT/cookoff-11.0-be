package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/timer"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
)

type questionQueries interface {
	questionReader
	ListQuestionsByRound(context.Context, int32) ([]sqlc.ListQuestionsByRoundRow, error)
	ListAllQuestions(context.Context) ([]sqlc.ListAllQuestionsRow, error)
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
func questionError(c echo.Context, s int, m string, err ...error) error {
	if s >= 500 {
		if len(err) > 0 && err[0] != nil {
			logging.Errorf("Question controller error [%d]: %s - %v", s, m, err[0])
		} else {
			logging.Errorf("Question controller error [%d]: %s", s, m)
		}
	}
	return c.JSON(s, dto.NewCodedError(m, dto.CodeForStatus(s)))
}

// roundOpened reports whether players may read round's questions: the contest
// clock has started that round (running or already over), or moved past it.
// Before that, even a qualified player gets nothing, so an early promotion
// does not leak the next round's problems.
func roundOpened(ctx context.Context, round int32) (bool, error) {
	status, err := timer.GetTime(ctx)
	if err != nil {
		return false, err
	}
	return status.Round > round || (status.Round == round && status.StartTime != nil), nil
}

// requireRoundOpened writes the error response and returns false when round
// has not been opened yet.
func requireRoundOpened(c echo.Context, round int32) bool {
	opened, err := roundOpened(c.Request().Context(), round)
	if err != nil {
		_ = questionError(c, http.StatusInternalServerError, "Failed to check round timer", err)
		return false
	}
	if !opened {
		_ = c.JSON(http.StatusLocked, dto.NewCodedError("This round has not started yet", dto.CodeRoundNotRunning))
		return false
	}
	return true
}

// contentTTL bounds how long players can see a question edit late if an
// invalidation is missed; admin writes invalidate immediately.
const contentTTL = 30 * time.Second

// visibleQuestion returns question qid when the signed-in user may read it:
// an admin always may (the admin panel edits questions of every round, before
// any round starts); a player only when it belongs to their round and that
// round has opened. It writes the error response and returns false otherwise.
func visibleQuestion(c echo.Context, queries questionReader, qid uuid.UUID) (dto.QuestionResponse, bool) {
	user, ok := middlewares.CurrentUser(c)
	if !ok {
		_ = questionError(c, http.StatusUnauthorized, "Unauthorized")
		return dto.QuestionResponse{}, false
	}
	q, e := cachedQuestion(c.Request().Context(), queries, qid)
	if e == nil && strings.EqualFold(user.Role, "admin") {
		return q, true
	}
	if errors.Is(e, pgx.ErrNoRows) || (e == nil && q.Round != user.RoundQualified) {
		_ = questionError(c, http.StatusNotFound, "Question not found")
		return dto.QuestionResponse{}, false
	}
	if e != nil {
		_ = questionError(c, http.StatusInternalServerError, "Failed to load question", e)
		return dto.QuestionResponse{}, false
	}
	if !requireRoundOpened(c, q.Round) {
		return dto.QuestionResponse{}, false
	}
	return q, true
}

type questionReader interface {
	GetQuestionByID(context.Context, uuid.UUID) (sqlc.GetQuestionByIDRow, error)
}

func cachedQuestion(ctx context.Context, queries questionReader, qid uuid.UUID) (dto.QuestionResponse, error) {
	return utils.Cached(ctx, utils.ContentCachePrefix+"question:"+qid.String(), contentTTL, func(ctx context.Context) (dto.QuestionResponse, error) {
		q, err := queries.GetQuestionByID(ctx, qid)
		if err != nil {
			return dto.QuestionResponse{}, err
		}
		return questionFromRow(q.ID, q.Description, q.Title, q.QType, q.InputFormat, q.BuyIn, q.Reward, q.Points, q.Round, q.Constraints, q.OutputFormat, q.SampleTestInput, q.SampleTestOutput, q.Explanation, q.BountyActive), nil
	})
}

func (qc *QuestionController) ListByRound(c echo.Context) error {
	user, ok := middlewares.CurrentUser(c)
	if !ok {
		return questionError(c, http.StatusUnauthorized, "Unauthorized")
	}
	if !requireRoundOpened(c, user.RoundQualified) {
		return nil
	}
	round := user.RoundQualified
	out, e := utils.Cached(c.Request().Context(), utils.ContentCachePrefix+"round:"+strconv.Itoa(int(round)), contentTTL, func(ctx context.Context) ([]dto.QuestionResponse, error) {
		rows, err := qc.queries.ListQuestionsByRound(ctx, round)
		if err != nil {
			return nil, err
		}
		out := make([]dto.QuestionResponse, len(rows))
		for i, q := range rows {
			out[i] = questionFromRow(q.ID, q.Description, q.Title, q.QType, q.InputFormat, q.BuyIn, q.Reward, q.Points, q.Round, q.Constraints, q.OutputFormat, q.SampleTestInput, q.SampleTestOutput, q.Explanation, q.BountyActive)
		}
		return out, nil
	})
	if e != nil {
		return questionError(c, http.StatusInternalServerError, "Failed to load questions", e)
	}
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Questions retrieved", out))
}
func (qc *QuestionController) ListAll(c echo.Context) error {
	rows, e := qc.queries.ListAllQuestions(c.Request().Context())
	if e != nil {
		return questionError(c, http.StatusInternalServerError, "Failed to load questions")
	}
	out := make([]dto.QuestionResponse, len(rows))
	for i, q := range rows {
		out[i] = questionFromRow(q.ID, q.Description, q.Title, q.QType, q.InputFormat, q.BuyIn, q.Reward, q.Points, q.Round, q.Constraints, q.OutputFormat, q.SampleTestInput, q.SampleTestOutput, q.Explanation, q.BountyActive)
	}
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Questions retrieved", out))
}

func (qc *QuestionController) GetByID(c echo.Context) error {
	qid, e := parseQuestionID(c)
	if e != nil {
		return questionError(c, http.StatusBadRequest, "Invalid question ID")
	}
	q, ok := visibleQuestion(c, qc.queries, qid)
	if !ok {
		return nil
	}
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Question retrieved", q))
}
func (qc *QuestionController) ListBlocks(c echo.Context) error {
	qid, e := parseQuestionID(c)
	if e != nil {
		return questionError(c, http.StatusBadRequest, "Invalid question ID")
	}
	q, ok := visibleQuestion(c, qc.queries, qid)
	if !ok {
		return nil
	}
	if q.Round != 1 || !strings.EqualFold(q.Type, "visual") {
		return questionError(c, http.StatusNotFound, "Round 1 visual question not found")
	}
	out, e := utils.Cached(c.Request().Context(), utils.ContentCachePrefix+"blocks:"+qid.String(), contentTTL, func(ctx context.Context) ([]dto.VisualBlockResponse, error) {
		blocks, err := qc.queries.ListVisualBlocksByQuestionID(ctx, qid)
		if err != nil {
			return nil, err
		}
		out := make([]dto.VisualBlockResponse, len(blocks))
		for i, b := range blocks {
			out[i] = dto.VisualBlockResponse{ID: b.ID, Content: b.Content}
		}
		return out, nil
	})
	if e != nil {
		return questionError(c, http.StatusInternalServerError, "Failed to load visual blocks", e)
	}
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Visual blocks retrieved", out))
}
func (qc *QuestionController) Create(c echo.Context) error {
	var r dto.QuestionRequest
	if e := c.Bind(&r); e != nil {
		return questionError(c, http.StatusBadRequest, "Invalid request body")
	}
	if e := c.Validate(&r); e != nil {
		return questionError(c, http.StatusBadRequest, "Validation failed")
	}
	q, e := qc.queries.CreateQuestion(c.Request().Context(), questionParams(uuid.New(), r))
	if e != nil {
		return questionError(c, http.StatusInternalServerError, "Failed to create question")
	}
	utils.InvalidateContentCache(c.Request().Context())
	return c.JSON(201, dto.NewSuccessResponse("Question created", questionFromModel(q)))
}
func (qc *QuestionController) Update(c echo.Context) error {
	id, e := parseQuestionID(c)
	if e != nil {
		return questionError(c, http.StatusBadRequest, "Invalid question ID")
	}
	var r dto.QuestionRequest
	if e = c.Bind(&r); e != nil {
		return questionError(c, http.StatusBadRequest, "Invalid request body")
	}
	if e = c.Validate(&r); e != nil {
		return questionError(c, http.StatusBadRequest, "Validation failed")
	}
	q, e := qc.queries.UpdateQuestion(c.Request().Context(), sqlc.UpdateQuestionParams(questionParams(id, r)))
	if errors.Is(e, pgx.ErrNoRows) {
		return questionError(c, http.StatusNotFound, "Question not found")
	}
	if e != nil {
		return questionError(c, http.StatusInternalServerError, "Failed to update question")
	}
	utils.InvalidateContentCache(c.Request().Context())
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Question updated", questionFromModel(q)))
}
func (qc *QuestionController) Delete(c echo.Context) error {
	id, e := parseQuestionID(c)
	if e != nil {
		return questionError(c, http.StatusBadRequest, "Invalid question ID")
	}
	_, e = qc.queries.DeleteQuestion(c.Request().Context(), id)
	if errors.Is(e, pgx.ErrNoRows) {
		return questionError(c, http.StatusNotFound, "Question not found")
	}
	if e != nil {
		return questionError(c, http.StatusInternalServerError, "Failed to delete question")
	}
	utils.InvalidateContentCache(c.Request().Context())
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Question deleted", nil))
}
func (qc *QuestionController) SetBounty(active bool) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, e := parseQuestionID(c)
		if e != nil {
			return questionError(c, http.StatusBadRequest, "Invalid question ID")
		}
		q, e := qc.queries.SetQuestionBountyActive(c.Request().Context(), sqlc.SetQuestionBountyActiveParams{ID: id, BountyActive: active})
		if errors.Is(e, pgx.ErrNoRows) {
			return questionError(c, http.StatusNotFound, "Question not found")
		}
		if e != nil {
			return questionError(c, http.StatusInternalServerError, "Failed to update bounty")
		}
		utils.InvalidateContentCache(c.Request().Context())
		return c.JSON(http.StatusOK, dto.NewSuccessResponse("Bounty updated", questionFromModel(q)))
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
