package middlewares

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/labstack/echo/v4"
)

// HTTPErrorHandler renders every returned error — middleware ones included —
// in the same {success,message,code} shape the controllers use, and never
// leaks an internal error string on a 5xx.
func HTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status := http.StatusInternalServerError
	message := "Internal server error"

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		status = httpErr.Code
		if status < http.StatusInternalServerError {
			message = fmt.Sprint(httpErr.Message)
		}
	}
	if status >= http.StatusInternalServerError {
		logging.Errorf("%s %s: %v", c.Request().Method, c.Request().URL.Path, err)
	}

	body := dto.NewCodedError(message, dto.CodeForStatus(status))
	var writeErr error
	if c.Request().Method == http.MethodHead {
		writeErr = c.NoContent(status)
	} else {
		writeErr = c.JSON(status, body)
	}
	if writeErr != nil {
		logging.Errorf("write error response: %v", writeErr)
	}
}
