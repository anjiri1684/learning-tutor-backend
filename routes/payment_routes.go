package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)


func PaymentRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	api.Post("/payments/webhook", handlers.HandlePaymentWebhook)
	
	paypal := api.Group("/payments/paypal", middleware.Protected())
	paypal.Post("/create-order/:paymentId", handlers.CreatePayPalOrderHandler)
	paypal.Post("/capture-order", handlers.CapturePayPalOrderHandler) 
}