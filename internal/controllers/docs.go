package controllers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

const scalarHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Cookoff 11.0 Backend API Docs</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <style>
        body { margin: 0; padding: 0; background-color: #0f172a; }
    </style>
</head>
<body>
    <script
        id="api-reference"
        type="application/json">
        {
            "spec": {
                "url": "/docs/docs.yaml"
            }
        }
    </script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.25.30"></script>
</body>
</html>`

func ServeDocs(c echo.Context) error {
	return c.HTML(http.StatusOK, scalarHTMLTemplate)
}
