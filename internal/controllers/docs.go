package controllers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func ServeDocs(c echo.Context) error {
	return c.String(http.StatusOK, "HEHE IF YOU KNOW ABT THIS ROUTE YOU KNOW HOW THIS WORKS.")
}
