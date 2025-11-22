package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func ResourceRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	teacher := api.Group("/teacher/bookings/:bookingId/resources", middleware.Protected(), middleware.TeacherRequired())
	teacher.Post("", handlers.UploadResource)

	student := api.Group("/bookings/:bookingId/resources", middleware.Protected())
	student.Get("", handlers.GetBookingResources)
}