package dto


type Judge0Submission struct {
	SourceCode       string  `json:"source_code"`
	LanguageID       int     `json:"language_id"`
	Stdin            string  `json:"stdin,omitempty"`
	ExpectedOutput   string  `json:"expected_output,omitempty"`
	ExecutionTimeout float64 `json:"cpu_time_limit"`
	Callback         string  `json:"callback_url,omitempty"`
}


// Judge0CallbackPayload is the JSON body Judge0 PUTs to /callback
// once a single testcase finishes executing.

//PLEASE change this to Judge0SubmissionResponse or something
type Judge0CallbackPayload struct {
	Token   string  `json:"token"`
	StdOut  *string `json:"stdout"`
	StdErr  *string `json:"stderr"`
	Message *string `json:"message"`
	Time    string  `json:"time"`   // seconds, as a string e.g. "0.045"
	Memory  int     `json:"memory"` // KB
	Status  struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	} `json:"status"`
}
