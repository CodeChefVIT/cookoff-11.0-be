package submission

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"

	"net/http"
	"net/url"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
)

//Contains stuff only related to submission request made to judge0

// returns a payload for a single submission
func CreateSubmissionPayload(sourceCode string, languageID int, testCase sqlc.Testcase) ([]byte, error) {
	execution_timeout_t, err := testCase.Runtime.Float64Value()
	if err != nil {
		return nil, err
	}

	//could do this in the controller as well (preferred)
	execution_timeout := execution_timeout_t.Float64 * utils.GetExecutionTimeMultiplier(languageID)
	if execution_timeout == 0 {
		return nil, errors.New("invalid languageID or execution_timeout")
	}

	submission := dto.Judge0Submission{
		SourceCode:       base64.StdEncoding.EncodeToString([]byte(sourceCode)),
		LanguageID:       languageID,
		Stdin:            base64.StdEncoding.EncodeToString([]byte(testCase.Input)),
		ExpectedOutput:   base64.StdEncoding.EncodeToString([]byte(testCase.ExpectedOutput)),
		ExecutionTimeout: execution_timeout, //would prefer ExecutionTimeout over Runtime
	}

	payload, err := json.Marshal(submission)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the payload: %w", err)
	}

	return payload, nil

}

// returns a payload for a batch submission of all the testcases
func CreateBatchSubmissionPayload(sourceCode string, languageID int, testCases []sqlc.Testcase) ([]byte, error) {
	submissions := make([]dto.Judge0Submission, len(testCases))

	callbackURL := utils.Config.CallbackURL
	if callbackURL == "" {
		return nil, errors.New("environment variable CALLBACK_URL not set")
	}

	for i, testcase := range testCases {
		execution_timeout_t, err := testcase.Runtime.Float64Value()
		if err != nil {
			return nil, err
		}

		//could do this in the controller as well (preferred)
		execution_timeout := execution_timeout_t.Float64 * utils.GetExecutionTimeMultiplier(languageID)
		if execution_timeout == 0 {
			return nil, errors.New("invalid languageID or execution_timeout")
		}

		submissions[i] = dto.Judge0Submission{
			SourceCode:       base64.StdEncoding.EncodeToString([]byte(sourceCode)),
			LanguageID:       languageID,
			Stdin:            base64.StdEncoding.EncodeToString([]byte(testcase.Input)),
			ExpectedOutput:   base64.StdEncoding.EncodeToString([]byte(testcase.ExpectedOutput)),
			ExecutionTimeout: execution_timeout, //would prefer ExecutionTimeout over Runtime
			Callback:         callbackURL,
		}
	}

	type batchRequest struct {
		Submissions []dto.Judge0Submission `json:"submissions"`
	}

	payload, err := json.Marshal(batchRequest{Submissions: submissions})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the payload: %w", err)
	}

	return payload, nil
}

// sends a payload to judge0 for evaluation
func SendBatchSubmissionPayload(client *http.Client, payload []byte) (*http.Response, error) {
	baseURI := utils.Config.Judge0URI
	if baseURI == "" {
		return nil, errors.New("environment variable JUDGE0_URI not set")
	}

	params := url.Values{}
	params.Add("base64_encoded", "true")
	params.Add("wait", "false")

	finalURI := baseURI + "/submissions/batch?" + params.Encode()

	req, err := http.NewRequest("POST", finalURI, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to send submission payload to judge0: %w", err)
	}

	return resp, nil

}

// sends a payload to judge0 for evaluation with wait=true, returns the response immediately
func SendSubmissionPayloadWithWait(client *http.Client, payload []byte) (*http.Response, error) {
	baseURI := utils.Config.Judge0URI
	if baseURI == "" {
		return nil, errors.New("environment variable JUDGE0_URI not set")
	}

	params := url.Values{}
	params.Add("base64_encoded", "true")
	params.Add("wait", "true")

	finalURI := baseURI + "/submissions?" + params.Encode()

	req, err := http.NewRequest("POST", finalURI, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to send submission payload to judge0: %w", err)
	}

	return resp, nil

}
