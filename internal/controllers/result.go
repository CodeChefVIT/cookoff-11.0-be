package controllers

import(
	"context"
	"time"
	"net/http"

	"github.com/google/uuid"

	"github.com/labstack/echo/v4"
)

func GetResult(c echo.Context) error {
	

	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Minute)
	defer cancel()

	submissionID, err := uuid.Parse(c.Param("submission_id"))
	if err!=nil{
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	//keep doing forever until timeout or successfully done

	//check status
	//need the result stuff here

	//fetch it if it is done


	return nil
}