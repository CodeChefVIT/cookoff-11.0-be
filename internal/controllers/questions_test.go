package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
)

type questionQuerierStub struct {
	getQuestion       sqlc.GetQuestionByIDRow
	getQuestionErr    error
	getVisualErr      error
	listQuestions     []sqlc.ListQuestionsByRoundRow
	listQuestionsErr  error
	listBlocks        []sqlc.VisualBlock
	listBlocksErr     error
	requestedRound    int32
	requestedQuestion uuid.UUID
	blocksRequested   bool
}

func (s *questionQuerierStub) CreateAttempt(_ context.Context, _ sqlc.CreateAttemptParams) (sqlc.Attempt, error) {
	return sqlc.Attempt{}, nil
}

func (s *questionQuerierStub) CreateSubmission(_ context.Context, _ sqlc.CreateSubmissionParams) error {
	return nil
}

func (s *questionQuerierStub) CreateTestCase(_ context.Context, _ sqlc.CreateTestCaseParams) (sqlc.Testcase, error) {
	return sqlc.Testcase{}, nil
}

func (s *questionQuerierStub) CreateVisualSubmission(_ context.Context, _ sqlc.CreateVisualSubmissionParams) (sqlc.Submission, error) {
	return sqlc.Submission{}, nil
}

func (s *questionQuerierStub) GetAllTestCasesByQuestion(_ context.Context, _ uuid.UUID) ([]sqlc.GetAllTestCasesByQuestionRow, error) {
	return nil, nil
}

func (s *questionQuerierStub) GetAttemptForUpdate(_ context.Context, _ sqlc.GetAttemptForUpdateParams) (sqlc.Attempt, error) {
	return sqlc.Attempt{}, nil
}

func (s *questionQuerierStub) GetQuestionBuyIn(_ context.Context, _ uuid.UUID) (pgtype.Numeric, error) {
	return pgtype.Numeric{}, nil
}

func (s *questionQuerierStub) GetQuestionReward(_ context.Context, _ uuid.UUID) (pgtype.Numeric, error) {
	return pgtype.Numeric{}, nil
}

func (s *questionQuerierStub) GetUserBalanceForUpdate(_ context.Context, _ uuid.UUID) (pgtype.Numeric, error) {
	return pgtype.Numeric{}, nil
}

func (s *questionQuerierStub) GetUserScoreForUpdate(_ context.Context, _ uuid.UUID) (pgtype.Numeric, error) {
	return pgtype.Numeric{}, nil
}

func (s *questionQuerierStub) ListVisualSolutionsByQuestionID(_ context.Context, _ uuid.UUID) ([]sqlc.VisualSolution, error) {
	return nil, nil
}

func (s *questionQuerierStub) UpdateAttemptStatus(_ context.Context, _ sqlc.UpdateAttemptStatusParams) error {
	return nil
}

func (s *questionQuerierStub) UpdateUserBalance(_ context.Context, _ sqlc.UpdateUserBalanceParams) error {
	return nil
}

func (s *questionQuerierStub) UpdateUserScore(_ context.Context, _ sqlc.UpdateUserScoreParams) error {
	return nil
}

func (s *questionQuerierStub) GetQuestionByID(_ context.Context, id uuid.UUID) (sqlc.GetQuestionByIDRow, error) {
	s.requestedQuestion = id
	return s.getQuestion, s.getQuestionErr
}

func (s *questionQuerierStub) GetRoundOneVisualQuestion(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	s.requestedQuestion = id
	return id, s.getVisualErr
}

func (s *questionQuerierStub) ListQuestionsByRound(_ context.Context, round int32) ([]sqlc.ListQuestionsByRoundRow, error) {
	s.requestedRound = round
	return s.listQuestions, s.listQuestionsErr
}

func (s *questionQuerierStub) ListVisualBlocksByQuestionID(_ context.Context, _ uuid.UUID) ([]sqlc.VisualBlock, error) {
	s.blocksRequested = true
	return s.listBlocks, s.listBlocksErr
}

func TestListByRoundReturnsOnlyRequestedRound(t *testing.T) {
	questionID := uuid.New()
	stub := &questionQuerierStub{listQuestions: []sqlc.ListQuestionsByRoundRow{{
		ID: questionID, Title: "Visual Logic", QType: "visual", Round: 1, BuyIn: "10", Reward: "20",
	}}}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/question/round?round=1", nil)
	res := httptest.NewRecorder()
	if err := NewQuestionController(stub).ListByRound(e.NewContext(req, res)); err != nil {
		t.Fatalf("ListByRound() error = %v", err)
	}

	if stub.requestedRound != 1 {
		t.Fatalf("requested round = %d, want 1", stub.requestedRound)
	}
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data := body["data"].([]interface{})
	if len(data) != 1 || data[0].(map[string]interface{})["round"].(float64) != 1 {
		t.Fatalf("unexpected response data: %v", body["data"])
	}
}

func TestGetByIDDoesNotExposeVisualSolutions(t *testing.T) {
	questionID := uuid.New()
	stub := &questionQuerierStub{getQuestion: sqlc.GetQuestionByIDRow{
		ID: questionID, Title: "Visual Logic", QType: "visual", Round: 1, BuyIn: "10", Reward: "20",
	}}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/question/"+questionID.String(), nil)
	res := httptest.NewRecorder()
	ctx := e.NewContext(req, res)
	ctx.SetPath("/question/:id")
	ctx.SetParamNames("id")
	ctx.SetParamValues(questionID.String())
	if err := NewQuestionController(stub).GetByID(ctx); err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data := body["data"].(map[string]interface{})
	if _, found := data["solution"]; found {
		t.Fatalf("response exposed a visual solution: %v", data)
	}
}

func TestListBlocksRejectsInvalidOrNonVisualQuestions(t *testing.T) {
	e := echo.New()
	controller := NewQuestionController(&questionQuerierStub{})
	invalidReq := httptest.NewRequest(http.MethodGet, "/question/not-a-uuid/blocks", nil)
	invalidRes := httptest.NewRecorder()
	invalidCtx := e.NewContext(invalidReq, invalidRes)
	invalidCtx.SetParamNames("id")
	invalidCtx.SetParamValues("not-a-uuid")
	if err := controller.ListBlocks(invalidCtx); err != nil {
		t.Fatalf("ListBlocks() invalid ID error = %v", err)
	}
	if invalidRes.Code != http.StatusBadRequest {
		t.Fatalf("invalid ID status = %d, want %d", invalidRes.Code, http.StatusBadRequest)
	}

	stub := &questionQuerierStub{getVisualErr: pgx.ErrNoRows}
	questionID := uuid.New()
	notVisualReq := httptest.NewRequest(http.MethodGet, "/question/"+questionID.String()+"/blocks", nil)
	notVisualRes := httptest.NewRecorder()
	notVisualCtx := e.NewContext(notVisualReq, notVisualRes)
	notVisualCtx.SetParamNames("id")
	notVisualCtx.SetParamValues(questionID.String())
	if err := NewQuestionController(stub).ListBlocks(notVisualCtx); err != nil {
		t.Fatalf("ListBlocks() non-visual error = %v", err)
	}
	if notVisualRes.Code != http.StatusNotFound {
		t.Fatalf("non-visual status = %d, want %d", notVisualRes.Code, http.StatusNotFound)
	}
	if stub.blocksRequested {
		t.Fatal("blocks query ran for a non-visual question")
	}
}

func TestListBlocksReturnsVisualPalette(t *testing.T) {
	questionID := uuid.New()
	stub := &questionQuerierStub{listBlocks: []sqlc.VisualBlock{{ID: uuid.New(), QuestionID: questionID, Content: "move 10 steps"}}}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/question/"+questionID.String()+"/blocks", nil)
	res := httptest.NewRecorder()
	ctx := e.NewContext(req, res)
	ctx.SetParamNames("id")
	ctx.SetParamValues(questionID.String())
	if err := NewQuestionController(stub).ListBlocks(ctx); err != nil {
		t.Fatalf("ListBlocks() error = %v", err)
	}
	if res.Code != http.StatusOK || !stub.blocksRequested {
		t.Fatalf("status = %d, blocks requested = %t", res.Code, stub.blocksRequested)
	}
}

func TestGetByIDReturnsNotFound(t *testing.T) {
	stub := &questionQuerierStub{getQuestionErr: pgx.ErrNoRows}
	questionID := uuid.New()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/question/"+questionID.String(), nil)
	res := httptest.NewRecorder()
	ctx := e.NewContext(req, res)
	ctx.SetParamNames("id")
	ctx.SetParamValues(questionID.String())
	if err := NewQuestionController(stub).GetByID(ctx); err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

var _ sqlc.Querier = (*questionQuerierStub)(nil)
