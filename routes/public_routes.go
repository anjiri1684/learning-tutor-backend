package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/gofiber/fiber/v2"
)

func PublicRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	api.Get("/locales/:lang", handlers.GetLocale)
	api.Get("/currency/rate", handlers.GetConversionRate) 

}