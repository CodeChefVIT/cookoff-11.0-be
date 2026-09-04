package submission

import(
	"encoding/base64"
	"encoding/json"
	"fmt"
	"errors"
	"bytes"
	"net/http"
	"net/url"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/utils"
)


//rename this

//Contains stuff only related to submission request made to judge0


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

	callbackURL := utils.Config.CallbackURL
	if callbackURL==""{
		return nil, errors.New("Environment Variable CALLBACK_URL not set")
	}

	for i, testcase := range testCases{
		execution_timeout_t, err := testcase.Runtime.Float64Value()
		if err!=nil{
			return nil, err
		}

		//could do this in the controller as well (preferred)
		execution_timeout := execution_timeout_t.Float64*utils.GetRuntimeMultiplier(languageID)
		if execution_timeout==0{
			return nil, errors.New("Invalid languageID or execution_timeout")
		}

		submissions[i]=judge0Submission{
			SourceCode : base64.StdEncoding.EncodeToString([]byte(sourceCode)),
			LanguageID : languageID,
			Stdin : base64.StdEncoding.EncodeToString([]byte(testcase.Input)),
			ExpectedOutput : base64.StdEncoding.EncodeToString([]byte(testcase.ExpectedOutput)),
			ExecutionTimeout : execution_timeout,//would prefer ExecutionTimeout over Runtime
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
func SendSubmissionPayload(client *http.Client, payload []byte) (*http.Response, error){
	baseURI := utils.Config.Judge0URI
	if baseURI==""{
		return nil, errors.New("Environment Variable JUDGE0_URI not set")
	}

	params := url.Values{}
	params.Add("base64_encoded", "true")

	finalURI := baseURI+"/submissions/batch?"+params.Encode()

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