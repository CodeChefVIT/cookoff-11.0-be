package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type fakeQuestions struct {
	questionQueries
	byID map[uuid.UUID]sqlc.GetQuestionByIDRow
}

func (f fakeQuestions) GetQuestionByID(_ context.Context, id uuid.UUID) (sqlc.GetQuestionByIDRow, error) {
	q, ok := f.byID[id]
	if !ok {
		return q, pgx.ErrNoRows
	}
	return q, nil
}

func getQuestion(t *testing.T, qc *QuestionController, qid uuid.UUID, user middlewares.AuthUser) int {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/question/"+qid.String(), nil), rec)
	c.SetParamNames("id")
	c.SetParamValues(qid.String())
	c.Set(middlewares.AuthUserKey, user)
	if err := qc.GetByID(c); err != nil {
		t.Fatal(err)
	}
	return rec.Code
}

func TestGetByIDAdminSeesAnyRoundBeforeItStarts(t *testing.T) {
	mr := miniredis.RunT(t)
	prev := utils.RedisClient
	utils.RedisClient = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { utils.RedisClient = prev })
	// Round 1 is set but not started; the question is a round 3 one.
	_ = mr.Set("contest:round", "1")

	qid := uuid.New()
	qc := NewQuestionController(fakeQuestions{byID: map[uuid.UUID]sqlc.GetQuestionByIDRow{
		qid: {ID: qid, Title: "Final", QType: "code", Round: 3, BuyIn: "", Reward: ""},
	}})

	if code := getQuestion(t, qc, qid, middlewares.AuthUser{ID: uuid.New(), Role: "admin", RoundQualified: 0}); code != http.StatusOK {
		t.Fatalf("admin got %d, want 200", code)
	}
	if code := getQuestion(t, qc, qid, middlewares.AuthUser{ID: uuid.New(), Role: "user", RoundQualified: 3}); code != http.StatusLocked {
		t.Fatalf("player before the round got %d, want 423", code)
	}
	if code := getQuestion(t, qc, qid, middlewares.AuthUser{ID: uuid.New(), Role: "user", RoundQualified: 2}); code != http.StatusNotFound {
		t.Fatalf("player from another round got %d, want 404", code)
	}
}
