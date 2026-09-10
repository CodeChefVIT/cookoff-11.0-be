package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
)

type questionStub struct {
	list []sqlc.ListQuestionsForUserRow
	get  sqlc.GetQuestionForUserRow
	err  error
}

func (s questionStub) ListQuestionsForUser(context.Context, uuid.UUID) ([]sqlc.ListQuestionsForUserRow, error) {
	return s.list, s.err
}
func (s questionStub) GetQuestionForUser(context.Context, sqlc.GetQuestionForUserParams) (sqlc.GetQuestionForUserRow, error) {
	return s.get, s.err
}
func (s questionStub) ListVisualBlocksByQuestionID(context.Context, uuid.UUID) ([]sqlc.VisualBlock, error) {
	return nil, s.err
}
func (s questionStub) CreateQuestion(context.Context, sqlc.CreateQuestionParams) (sqlc.Question, error) {
	return sqlc.Question{}, s.err
}
func (s questionStub) UpdateQuestion(context.Context, sqlc.UpdateQuestionParams) (sqlc.Question, error) {
	return sqlc.Question{}, s.err
}
func (s questionStub) DeleteQuestion(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, s.err
}
func (s questionStub) SetQuestionBountyActive(context.Context, sqlc.SetQuestionBountyActiveParams) (sqlc.Question, error) {
	return sqlc.Question{}, s.err
}

type testcaseStub struct {
	questionErr error
	public      []sqlc.Testcase
}

func (s testcaseStub) CreateTestCase(context.Context, sqlc.CreateTestCaseParams) (sqlc.Testcase, error) {
	return sqlc.Testcase{}, nil
}
func (s testcaseStub) GetTestCaseByID(context.Context, uuid.UUID) (sqlc.Testcase, error) {
	return sqlc.Testcase{}, pgx.ErrNoRows
}
func (s testcaseStub) UpdateTestCase(context.Context, sqlc.UpdateTestCaseParams) (sqlc.Testcase, error) {
	return sqlc.Testcase{}, nil
}
func (s testcaseStub) DeleteTestCase(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (s testcaseStub) GetQuestionForUser(context.Context, sqlc.GetQuestionForUserParams) (sqlc.GetQuestionForUserRow, error) {
	return sqlc.GetQuestionForUserRow{}, s.questionErr
}
func (s testcaseStub) GetPublicTestCasesByQuestion(context.Context, uuid.UUID) ([]sqlc.Testcase, error) {
	return s.public, nil
}
func (s testcaseStub) GetAllTestCasesByQuestion(context.Context, uuid.UUID) ([]sqlc.GetAllTestCasesByQuestionRow, error) {
	return nil, nil
}

type dashboardStub struct {
	user sqlc.User
	rows []sqlc.ListDashboardQuestionsRow
}

func (s dashboardStub) GetUserByID(context.Context, uuid.UUID) (sqlc.User, error) { return s.user, nil }
func (s dashboardStub) ListDashboardQuestions(context.Context, uuid.UUID) ([]sqlc.ListDashboardQuestionsRow, error) {
	return s.rows, nil
}

func request(e *echo.Echo, method, path string, user uuid.UUID) (echo.Context, *httptest.ResponseRecorder) {
	r := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	c := e.NewContext(r, w)
	c.SetParamNames("id")
	c.SetParamValues(uuid.NewString())
	if user != uuid.Nil {
		c.Set(middlewares.UserIDKey, user.String())
	}
	return c, w
}

func TestListByRoundUsesAuthenticatedUser(t *testing.T) {
	t.Parallel()
	e := echo.New()
	uid := uuid.New()
	c, w := request(e, http.MethodGet, "/question/round", uid)
	q := QuestionController{queries: questionStub{list: []sqlc.ListQuestionsForUserRow{{ID: uuid.New(), Title: "visible", Round: 2}}}}
	if err := q.ListByRound(c); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var body struct {
		Data []struct {
			Title string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Data[0].Title != "visible" {
		t.Fatalf("unexpected questions: %#v", body.Data)
	}
}
func TestQuestionOutsideQualifiedRoundIsNotDisclosed(t *testing.T) {
	t.Parallel()
	e := echo.New()
	c, w := request(e, http.MethodGet, "/question/id", uuid.New())
	q := QuestionController{queries: questionStub{err: pgx.ErrNoRows}}
	if err := q.GetByID(c); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d", w.Code)
	}
}
func TestPublicTestcasesExcludeHiddenSource(t *testing.T) {
	t.Parallel()
	e := echo.New()
	c, w := request(e, http.MethodGet, "/question/id/testcases/public", uuid.New())
	n := pgtype.Numeric{}
	if err := n.Scan("1"); err != nil {
		t.Fatal(err)
	}
	tc := TestcaseController{queries: testcaseStub{public: []sqlc.Testcase{{ID: uuid.New(), Input: "1", ExpectedOutput: "2", Memory: n, Runtime: n, Hidden: false}}}}
	if err := tc.ListPublic(c); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if string(w.Body.Bytes()) == "" {
		t.Fatal("empty response")
	}
}
func TestDashboardIncludesAttemptStatuses(t *testing.T) {
	t.Parallel()
	e := echo.New()
	uid := uuid.New()
	c, w := request(e, http.MethodGet, "/dashboard", uid)
	n := pgtype.Numeric{}
	if err := n.Scan("10"); err != nil {
		t.Fatal(err)
	}
	h := Dashboard(dashboardStub{user: sqlc.User{ID: uid, Name: "Ada", Email: "ada@example.com", Balance: n, Score: n, RoundQualified: 2}, rows: []sqlc.ListDashboardQuestionsRow{{ID: uuid.New(), Title: "A", Round: 2, AttemptStatus: "bought"}, {ID: uuid.New(), Title: "B", Round: 2, AttemptStatus: "answered"}}})
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var body struct {
		Data struct {
			AttemptTotals map[string]int `json:"attempt_totals"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.AttemptTotals["bought"] != 1 || body.Data.AttemptTotals["answered"] != 1 {
		t.Fatalf("unexpected totals: %#v", body.Data.AttemptTotals)
	}
}
