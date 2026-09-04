package controllers

import(
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/submission"

	"github.com/labstack/echo/v4"
	"github.com/google/uuid"
	"net/http"
	"fmt"
)

//logging not done
func SubmitCode(c echo.Context) error {
	var req dto.SubmissionRequest
	if err := c.Bind(&req); err!=nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	//get user id as well here
	//userID := 


	//parsing stuff here
	//here

	//auth stuff
	//here


	submissionID := uuid.New()
	fmt.Errorf("%v", submissionID)

	ctx := c.Request().Context()

	//fetch testcases from db
	testcases, err := db.GetAllTestCasesByQuestion(ctx, req.QuestionID)
	
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
	
	if resp.StatusCode!=nil{
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
	err := json.Unmarshal(body, &tokens)

	
	for i, t:= range tokens{
		//add the token to the queue and all.....
	}


	err := db.CreateSubmission(ctx, db.CreateSubmissionParams{
		ID: submissionID,
		QuestionID: req.QuestionID,
		SourceCode: req.SourceCode,
		LanguageID: req.LanguageID,
		//UserID: userID,
		//other stuff as well
	})
	if err!=nil{
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create submission in database"})
	}


	return c.JSON(http.StatusOK, echo.Map{
		"submission_id": submissionID,
	})
}
