
package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/gofiber/fiber/v2"
)

func AuthRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	auth := api.Group("/auth")
	auth.Post("/register", handlers.RegisterUser)
	auth.Post("/login", handlers.LoginUser) 
	auth.Post("/forgot-password", handlers.ForgotPassword) 
	auth.Post("/reset-password", handlers.ResetPassword)   
}