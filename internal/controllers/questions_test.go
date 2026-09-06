package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

type questionQuerierStub struct {
	getQuestion       db.GetQuestionByIDRow
	getQuestionErr    error
	getVisualErr      error
	listQuestions     []db.ListQuestionsByRoundRow
	listQuestionsErr  error
	listBlocks        []db.VisualBlock
	listBlocksErr     error
	requestedRound    int32
	requestedQuestion uuid.UUID
	blocksRequested   bool
}

func (s *questionQuerierStub) GetQuestionByID(_ context.Context, id uuid.UUID) (db.GetQuestionByIDRow, error) {
	s.requestedQuestion = id
	return s.getQuestion, s.getQuestionErr
}

func (s *questionQuerierStub) GetRoundOneVisualQuestion(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	s.requestedQuestion = id
	return id, s.getVisualErr
}

func (s *questionQuerierStub) ListQuestionsByRound(_ context.Context, round int32) ([]db.ListQuestionsByRoundRow, error) {
	s.requestedRound = round
	return s.listQuestions, s.listQuestionsErr
}

func (s *questionQuerierStub) ListVisualBlocksByQuestionID(_ context.Context, _ uuid.UUID) ([]db.VisualBlock, error) {
	s.blocksRequested = true
	return s.listBlocks, s.listBlocksErr
}

func TestListByRoundReturnsOnlyRequestedRound(t *testing.T) {
	questionID := uuid.New()
	stub := &questionQuerierStub{listQuestions: []db.ListQuestionsByRoundRow{{
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
	stub := &questionQuerierStub{getQuestion: db.GetQuestionByIDRow{
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
	stub := &questionQuerierStub{listBlocks: []db.VisualBlock{{ID: uuid.New(), QuestionID: questionID, Content: "move 10 steps"}}}
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

var _ db.Querier = (*questionQuerierStub)(nil)
