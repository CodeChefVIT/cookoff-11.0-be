package controllers

import (
	"net/http"

	"github.com/MarceloPetrucio/go-scalar-api-reference"
	"github.com/labstack/echo/v4"
)

func ServeDocs(c echo.Context) error {
	content, err := scalar.ApiReferenceHTML(&scalar.Options{
		SpecURL: "./docs/docs.yaml",
		CustomOptions: scalar.CustomOptions{
			PageTitle: "Cookoff 11.0 Backend API Docs",
		},
		DarkMode: true,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
	}

	return c.HTML(http.StatusOK, content)
}
