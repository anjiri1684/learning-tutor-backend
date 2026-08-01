package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func WaitlistRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	waitlist := api.Group("/availability/:slotId/waitlist", middleware.Protected())
	waitlist.Post("", handlers.JoinWaitlist)
	waitlist.Delete("", handlers.LeaveWaitlist)

	api.Get("/waitlist/me", middleware.Protected(), handlers.ListMyWaitlist)
}
