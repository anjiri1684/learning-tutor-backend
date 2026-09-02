package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func UploadRoutes(app *fiber.App) {
	// Scope the auth middleware to /uploads only. Using app.Group("/api/v1",
	// Protected()) here leaked auth onto every /api/v1 route registered after
	// this file in main.go.
	uploads := app.Group("/api/v1/uploads", middleware.Protected())
	uploads.Get("/signature", handlers.GenerateUploadSignature)
}
