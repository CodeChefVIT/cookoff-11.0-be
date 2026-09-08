package controllers
import(
	"net/http"
	"io"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/submission"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"

)

func RunCode(c echo.Context) error {

	var req dto.SubmissionRequest
	
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	//get user id here
	//userID :=

	//auth stuff
	//here


	questionID, err := uuid.Parse(req.QuestionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	submissionID := uuid.New()
	logging.Infof("Created submission ID: %v", submissionID)


	ctx := c.Request().Context()

	//fetch testcases from db
	testcases, err := db.Queries.GetPublicTestCasesByQuestion(ctx, questionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	result := make([]dto.Judge0CallbackPayload, len(testcases))


	client := &http.Client{}

	for i, testcase := range testcases {

		payload, err := submission.CreateSubmissionPayload(req.SourceCode, req.LanguageID, testcase)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}


		resp, err := submission.SendSubmissionPayloadWithWait(client, payload)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		defer resp.Body.Close()

		/*
		if resp.StatusCode != http.StatusCreated {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failure at Judge0"})
		}*/


		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error reading response body"})
		}

		json.Unmarshal(body, &result[i])

	}


	return c.JSON(http.StatusOK, result)
}


func RunCustom(c echo.Context) error {
	

	return nil
	//return c.JSON(http.StatusOK, result)
}