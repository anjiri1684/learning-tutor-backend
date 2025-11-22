package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func ProfileRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	profile := api.Group("/profile/me", middleware.Protected()) 
	profile.Get("", handlers.GetProfile)
	profile.Put("", handlers.UpdateProfile)
	profile.Get("/progress", handlers.GetMyProgress)

}