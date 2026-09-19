package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
)

type testcaseQueries interface {
	CreateTestCase(context.Context, sqlc.CreateTestCaseParams) (sqlc.Testcase, error)
	GetTestCaseByID(context.Context, uuid.UUID) (sqlc.Testcase, error)
	UpdateTestCase(context.Context, sqlc.UpdateTestCaseParams) (sqlc.Testcase, error)
	DeleteTestCase(context.Context, uuid.UUID) (uuid.UUID, error)
	questionReader
	GetPublicTestCasesByQuestion(context.Context, uuid.UUID) ([]sqlc.Testcase, error)
	GetAllTestCasesByQuestion(context.Context, uuid.UUID) ([]sqlc.Testcase, error)
}

type TestcaseController struct{ queries testcaseQueries }

func NewTestcaseController(q testcaseQueries) *TestcaseController {
	return &TestcaseController{queries: q}
}
func testcaseError(c echo.Context, s int, m string, err ...error) error {
	if s >= 500 {
		if len(err) > 0 && err[0] != nil {
			logging.Errorf("Testcase controller error [%d]: %s - %v", s, m, err[0])
		} else {
			logging.Errorf("Testcase controller error [%d]: %s", s, m)
		}
	}
	return c.JSON(s, dto.NewCodedError(m, dto.CodeForStatus(s)))
}
func (tc *TestcaseController) Create(c echo.Context) error {
	var r dto.TestcaseRequest
	if e := c.Bind(&r); e != nil {
		return testcaseError(c, http.StatusBadRequest, "Invalid request body")
	}
	if e := c.Validate(&r); e != nil {
		return testcaseError(c, http.StatusBadRequest, "Validation failed")
	}
	qid, e := uuid.Parse(r.QuestionID)
	if e != nil {
		return testcaseError(c, http.StatusBadRequest, "Invalid question ID")
	}
	m, e := utils.Float64ToNumeric(r.Memory)
	if e != nil {
		return testcaseError(c, http.StatusBadRequest, "Invalid memory")
	}
	rt, e := utils.Float64ToNumeric(r.Runtime)
	if e != nil {
		return testcaseError(c, http.StatusBadRequest, "Invalid runtime")
	}
	v, e := tc.queries.CreateTestCase(c.Request().Context(), sqlc.CreateTestCaseParams{ID: uuid.New(), ExpectedOutput: r.ExpectedOutput, Memory: m, Input: r.Input, Hidden: r.Hidden, Runtime: rt, QuestionID: qid})
	if e != nil {
		return testcaseError(c, http.StatusInternalServerError, "Failed to create testcase")
	}
	utils.InvalidateContentCache(c.Request().Context())
	return c.JSON(http.StatusCreated, dto.NewSuccessResponse("Testcase created", testcaseResponse(v)))
}
func (tc *TestcaseController) Update(c echo.Context) error {
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		return testcaseError(c, http.StatusBadRequest, "Invalid testcase ID")
	}
	old, e := tc.queries.GetTestCaseByID(c.Request().Context(), id)
	if errors.Is(e, pgx.ErrNoRows) {
		return testcaseError(c, http.StatusNotFound, "Testcase not found")
	}
	if e != nil {
		return testcaseError(c, http.StatusInternalServerError, "Failed to load testcase")
	}
	var r dto.TestcaseUpdateRequest
	if e = c.Bind(&r); e != nil {
		return testcaseError(c, http.StatusBadRequest, "Invalid request body")
	}
	if e = c.Validate(&r); e != nil {
		return testcaseError(c, http.StatusBadRequest, "Validation failed")
	}
	v := sqlc.UpdateTestCaseParams{ID: id, ExpectedOutput: old.ExpectedOutput, Memory: old.Memory, Input: old.Input, Hidden: old.Hidden, Runtime: old.Runtime, QuestionID: old.QuestionID}
	if r.ExpectedOutput != nil {
		v.ExpectedOutput = *r.ExpectedOutput
	}
	if r.Input != nil {
		v.Input = *r.Input
	}
	if r.Hidden != nil {
		v.Hidden = *r.Hidden
	}
	if r.Memory != nil {
		v.Memory, _ = utils.Float64ToNumeric(*r.Memory)
	}
	if r.Runtime != nil {
		v.Runtime, _ = utils.Float64ToNumeric(*r.Runtime)
	}
	if r.QuestionID != nil {
		v.QuestionID, e = uuid.Parse(*r.QuestionID)
		if e != nil {
			return testcaseError(c, http.StatusBadRequest, "Invalid question ID")
		}
	}
	updated, e := tc.queries.UpdateTestCase(c.Request().Context(), v)
	if e != nil {
		return testcaseError(c, http.StatusInternalServerError, "Failed to update testcase")
	}
	utils.InvalidateContentCache(c.Request().Context())
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Testcase updated", testcaseResponse(updated)))
}
func (tc *TestcaseController) Delete(c echo.Context) error {
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		return testcaseError(c, http.StatusBadRequest, "Invalid testcase ID")
	}
	_, e = tc.queries.DeleteTestCase(c.Request().Context(), id)
	if errors.Is(e, pgx.ErrNoRows) {
		return testcaseError(c, http.StatusNotFound, "Testcase not found")
	}
	if e != nil {
		return testcaseError(c, http.StatusInternalServerError, "Failed to delete testcase")
	}
	utils.InvalidateContentCache(c.Request().Context())
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Testcase deleted", nil))
}

func (tc *TestcaseController) Get(c echo.Context) error {
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		return testcaseError(c, http.StatusBadRequest, "Invalid testcase ID")
	}
	v, e := tc.queries.GetTestCaseByID(c.Request().Context(), id)
	if errors.Is(e, pgx.ErrNoRows) {
		return testcaseError(c, http.StatusNotFound, "Testcase not found")
	}
	if e != nil {
		return testcaseError(c, http.StatusInternalServerError, "Failed to load testcase")
	}
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Testcase retrieved", testcaseResponse(v)))
}
func (tc *TestcaseController) ListPublic(c echo.Context) error {
	qid, e := parseQuestionID(c)
	if e != nil {
		return testcaseError(c, http.StatusBadRequest, "Invalid question ID")
	}
	if _, ok := visibleQuestion(c, tc.queries, qid); !ok {
		return nil
	}
	out, e := utils.Cached(c.Request().Context(), utils.ContentCachePrefix+"testcases:"+qid.String(), contentTTL, func(ctx context.Context) ([]dto.TestcaseResponse, error) {
		rows, err := tc.queries.GetPublicTestCasesByQuestion(ctx, qid)
		if err != nil {
			return nil, err
		}
		out := make([]dto.TestcaseResponse, len(rows))
		for i, v := range rows {
			out[i] = testcaseResponse(v)
		}
		return out, nil
	})
	if e != nil {
		return testcaseError(c, http.StatusInternalServerError, "Failed to load testcases", e)
	}
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Public testcases retrieved", out))
}
func (tc *TestcaseController) ListAllForQuestion(c echo.Context) error {
	qid, e := parseQuestionID(c)
	if e != nil {
		return testcaseError(c, http.StatusBadRequest, "Invalid question ID")
	}
	rows, e := tc.queries.GetAllTestCasesByQuestion(c.Request().Context(), qid)
	if e != nil {
		return testcaseError(c, http.StatusInternalServerError, "Failed to load testcases")
	}
	out := make([]dto.TestcaseResponse, len(rows))
	for i, v := range rows {
		out[i] = dto.TestcaseResponse{ID: v.ID, QuestionID: v.QuestionID, ExpectedOutput: v.ExpectedOutput, Input: v.Input, Memory: numericText(v.Memory), Runtime: numericText(v.Runtime), Hidden: v.Hidden}
	}
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Testcases retrieved", out))
}
func testcaseResponse(v sqlc.Testcase) dto.TestcaseResponse {
	return dto.TestcaseResponse{ID: v.ID, QuestionID: v.QuestionID, ExpectedOutput: v.ExpectedOutput, Input: v.Input, Memory: numericText(v.Memory), Runtime: numericText(v.Runtime), Hidden: v.Hidden}
}
func numericText(n pgtype.Numeric) string {
	f, e := utils.NumericToFloat64(n)
	if e != nil {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
