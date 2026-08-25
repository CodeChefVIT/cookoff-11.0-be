package submission

import(
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"errors"
	"bytes"
	"net/http"
	"net/url"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
)


//intentionally left private
type judge0Submission struct{
	SourceCode string `json:"source_code"`
	LanguageID int `json:"language_id"`
	Stdin string `json:"stdin,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
	ExecutionTimeout float64 `json:"cpu_time_limit"`
	Callback string `json:"callback_url"`

}


//returns a payload for a batch submission of all the testcases
func CreateSubmissionPayload(sourceCode string, languageID int, testCases []db.Testcase) ([]byte, error){
	submissions := make([]judge0Submission, len(testCases))

	callbackURL := os.Getenv("CALLBACK_URL")
	if callbackURL==""{
		return nil, errors.New("Environment Variable CALLBACK_URL not set")
	}

	for i, testcase := range testCases{
		execution_timeout, err := testcase.Runtime.Float64Value()
		if err!=nil{
			return nil, err
		}

		//do runtime multiplier thing here
		//or in the controller(preferred)

		submissions[i]=judge0Submission{
			SourceCode : base64.StdEncoding.EncodeToString([]byte(sourceCode)),
			LanguageID : languageID,
			Stdin : base64.StdEncoding.EncodeToString([]byte(testcase.Input)),
			ExpectedOutput : base64.StdEncoding.EncodeToString([]byte(testcase.ExpectedOutput)),
			ExecutionTimeout : execution_timeout.Float64,//would prefer ExecutionTimeout over Runtime
			Callback : callbackURL,
		}
	}

	payload, err := json.Marshal(submissions)
	if err!=nil{
		return nil, fmt.Errorf("Failed to marshal the payload: %v", err)
	}

	return payload, nil
}


//sends a payload to judge0 for evaluation
func SendSubmissionPayload(payload []byte) (*http.Response, error){
	baseURI := os.Getenv("JUDGE0_URI")
	if baseURI==""{
		return nil, errors.New("Environment Variable JUDGE0_URI not set")
	}

	params := url.Values{}
	params.Add("base64_encoded", "true")

	finalURI := baseURI+"/submissions/batch?"+params.Encode()

	client := &http.Client{}

	req, err := http.NewRequest("POST", finalURI, bytes.NewReader(payload))
	if err!=nil{
		return nil, fmt.Errorf("Failed to create http request")
	}

	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)

	if err!=nil{
		return nil, fmt.Errorf("Failed to send submission payload to judge0")
	}

	return resp, nil

}