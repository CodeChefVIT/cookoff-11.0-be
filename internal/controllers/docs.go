package controllers

import (
	"net/http"
	"os"

	scalar "github.com/MarceloPetrucio/go-scalar-api-reference"
	"github.com/labstack/echo/v4"
)

func ServeDocs(c echo.Context) error {
	specBytes, _ := os.ReadFile("docs/docs.yaml")
	content, err := scalar.ApiReferenceHTML(&scalar.Options{
		SpecURL:     "/docs/docs.yaml",
		SpecContent: string(specBytes),
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
