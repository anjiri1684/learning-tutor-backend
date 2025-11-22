package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)



func BookingRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	booking := api.Group("/bookings", middleware.Protected())
	booking.Get("/me", handlers.GetMyBookings)
	booking.Post("", handlers.CreateBooking)
	booking.Post("/:bookingId/review", handlers.CreateReview) 
	booking.Post("/:bookingId/request-refund", handlers.RequestRefund) 
	booking.Post("/:bookingId/request-reschedule", handlers.RequestReschedule)

	teacherBooking := api.Group("/teacher/bookings", middleware.Protected(), middleware.TeacherRequired())
	teacherBooking.Post("/:bookingId/complete", handlers.MarkBookingAsComplete)
	teacherBooking.Post("/:bookingId/feedback", handlers.SubmitTeacherFeedback)
}