package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func PaymentRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	api.Post("/payments/webhook", middleware.KCBWebhookIPAllowlist(), handlers.HandlePaymentWebhook)

	// Public, signature-checked. Its path must not start with
	// "/payments/paystack": Fiber applies group middleware by plain string
	// prefix, so that group's Protected() would also cover it.
	api.Post("/payments/webhooks/paystack", handlers.PaystackWebhookHandler)

	paystack := api.Group("/payments/paystack", middleware.Protected())
	paystack.Post("/verify", handlers.VerifyPaystackPaymentHandler)
}
