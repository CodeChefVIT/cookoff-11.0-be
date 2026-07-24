package controllers

import (
	"net/http"

	"github.com/MarceloPetrucio/go-scalar-api-reference"
	"github.com/labstack/echo/v4"
)

func ServeDocs(c echo.Context) error {
	content, err := scalar.ApiReferenceHTML(&scalar.Options{
		SpecURL: "/docs/swagger.yaml",
		CustomOptions: scalar.CustomOptions{
			PageTitle: "Go Backend Template API Docs",
		},
		DarkMode: true,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load API documentation")
	}

	return c.HTML(http.StatusOK, content)
}
