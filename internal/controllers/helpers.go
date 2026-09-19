package controllers

import (
	"errors"
	"fmt"

	"github.com/labstack/echo/v4"
)

// validationMessage turns a validator error into the message shown to clients.
func validationMessage(err error) string {
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		return fmt.Sprint(httpErr.Message)
	}
	return "Invalid request"
}
