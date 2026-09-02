package routes

import (
	"time"

	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

func AuthRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	auth := api.Group("/auth")

	// Brute-force guard: these endpoints accept a password or reset token
	// guess with no other gate, so they're the obvious target for automated
	// credential-stuffing / token-guessing attacks.
	loginLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return middleware.ClientIP(c)
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "Too many attempts. Please try again later."})
		},
	})
	passwordResetLimiter := limiter.New(limiter.Config{
		Max:        5,
		Expiration: 15 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return middleware.ClientIP(c)
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "Too many attempts. Please try again later."})
		},
	})

	auth.Post("/register", loginLimiter, handlers.RegisterUser)
	auth.Post("/login", loginLimiter, handlers.LoginUser)
	auth.Post("/forgot-password", passwordResetLimiter, handlers.ForgotPassword)
	auth.Post("/reset-password", passwordResetLimiter, handlers.ResetPassword)
}
