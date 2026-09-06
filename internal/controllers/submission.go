package controllers

import(
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/submission"

	"github.com/labstack/echo/v4"
	"github.com/google/uuid"
	"net/http"
	"fmt"
	"io"
	"encoding/json"
)

//do logging
func SubmitCode(c echo.Context) error {
	var req dto.SubmissionRequest
	if err := c.Bind(&req); err!=nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	c.Validate(req)

	//get user id here
	//userID := 


	//auth stuff
	//here


	questionID, err := uuid.Parse(req.QuestionID)
	if err!=nil{
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	submissionID := uuid.New()
	fmt.Errorf("%v", submissionID)

	ctx := c.Request().Context()

	//fetch testcases from db
	testcases, err := db.Queries.GetAllTestCasesByQuestion(ctx, questionID)
	if err!=nil{
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	//make payload
	payload, err := submission.CreateSubmissionPayload(req.SourceCode, req.LanguageID, testcases)
	if err!=nil{
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	client := &http.Client{}

	//send the payload
	resp, err := submission.SendSubmissionPayload(client, payload)
	if err!=nil{
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	
	defer resp.Body.Close()
	
	if resp.StatusCode!=http.StatusCreated{
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failure at Judge0"})
	}
	body, err := io.ReadAll(resp.Body)
	if err!=nil{
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error reading response body"})
	}



	type Token struct {
		Token string `json:"token"`
	}

	var tokens []Token
	err = json.Unmarshal(body, &tokens)
	if err!=nil{
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to decode body"})
	}

	
	/*
	for i, t:= range tokens{
		//add the token to the queue and all.....
	}*/


	err = db.Queries.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		ID: submissionID,
		QuestionID: questionID,
		SourceCode: req.SourceCode,
		LanguageID: int32(req.LanguageID),
		//UserID: userID,
		//anything else if required
	})
	if err!=nil{
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create submission in database"})
	}


	return c.JSON(http.StatusOK, echo.Map{
		"submission_id": submissionID,
	})
}
