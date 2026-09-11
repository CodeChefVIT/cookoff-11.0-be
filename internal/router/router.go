package router

import (
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/controllers"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo) {
	queries := sqlc.New(db.DBPool)
	authController := controllers.NewAuthController(queries)
	authenticated := []echo.MiddlewareFunc{middlewares.VerifyJWTMiddleware, middlewares.BanCheckUser(queries)}

	// Standard operational routes
	e.GET("/health", controllers.HealthCheck)
	e.GET("/docs", controllers.ServeDocs)

	// judge0 callback req
	e.PUT("/judge0callback", controllers.Judge0Callback)
	e.GET("/auth/google", authController.StartGoogle)
	e.GET("/auth/google/callback", authController.GoogleCallback)
	// Keep the legacy versioned OAuth paths working for existing Google Console
	// redirect URIs while new clients use the unversioned contract.
	e.GET("/api/v1/auth/google", authController.StartGoogle)
	e.GET("/api/v1/auth/google/callback", authController.GoogleCallback)
	e.POST("/refreshToken", authController.RefreshToken)
	e.POST("/logout", authController.Logout, middlewares.VerifyJWTMiddleware)

	// submit & result routes
	e.POST("/submit", controllers.SubmitCode, authenticated...)
	e.GET("/result/:submission_id", controllers.GetResult, authenticated...)

	RegisterAttemptRoutes(e, authenticated...)
	RegisterVisualSubmissionRoutes(e, authenticated...)

	questionController := controllers.NewQuestionController(queries)
	questionRoutes := e.Group("/question", authenticated...)
	questionRoutes.GET("/round", questionController.ListByRound)
	questionRoutes.GET("/:id", questionController.GetByID)
	questionRoutes.GET("/:id/blocks", questionController.ListBlocks)
	questionRoutes.GET("/:id/testcases/public", controllers.NewTestcaseController(queries).ListPublic)

	adminQuestionRoutes := e.Group("/question", authenticated[0], authenticated[1], middlewares.AdminOnly)
	adminQuestionRoutes.POST("", questionController.Create)
	adminQuestionRoutes.PUT("/:id", questionController.Update)
	adminQuestionRoutes.DELETE("/:id", questionController.Delete)
	adminQuestionRoutes.POST("/:id/bounty/activate", questionController.SetBounty(true))
	adminQuestionRoutes.POST("/:id/bounty/deactivate", questionController.SetBounty(false))
	adminQuestionRoutes.GET("/:id/testcases", controllers.NewTestcaseController(queries).ListAllForQuestion)

	testcaseController := controllers.NewTestcaseController(queries)
	adminTestcaseRoutes := e.Group("/testcase", authenticated[0], authenticated[1], middlewares.AdminOnly)
	adminTestcaseRoutes.GET("/:id", testcaseController.Get)
	adminTestcaseRoutes.POST("", testcaseController.Create)
	adminTestcaseRoutes.PUT("/:id", testcaseController.Update)
	adminTestcaseRoutes.DELETE("/:id", testcaseController.Delete)

	e.GET("/dashboard", controllers.Dashboard(queries), authenticated...)
	e.GET("/admin/session", controllers.AdminSession(queries), authenticated[0], authenticated[1], middlewares.AdminOnly)
}
