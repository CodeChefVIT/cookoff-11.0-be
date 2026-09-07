package dto

// Judge0CallbackPayload is the JSON body Judge0 PUTs to /callback
// once a single testcase finishes executing.
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
