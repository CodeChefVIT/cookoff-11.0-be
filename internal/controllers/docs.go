package controllers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

const scalarHTML = `<!DOCTYPE html>
<html>
  <head>
    <title>Cookoff 11.0 Backend API Docs</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script
      id="api-reference"
      data-url="/docs.yaml">
    </script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.25.30/dist/browser/standalone.js"></script>
  </body>
</html>`

func ServeDocs(c echo.Context) error {
	return c.HTML(http.StatusOK, scalarHTML)
}
