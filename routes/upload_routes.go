package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func UploadRoutes(app *fiber.App) {
	api := app.Group("/api/v1", middleware.Protected()) 

	uploads := api.Group("/uploads")
	uploads.Get("/signature", handlers.GenerateUploadSignature)
}