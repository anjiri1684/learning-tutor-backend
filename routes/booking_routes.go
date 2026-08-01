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
	booking.Get("/me/calendar.ics", handlers.GetMyCalendar)
	booking.Get("/:bookingId/calendar.ics", handlers.GetBookingCalendar)
	booking.Post("", handlers.CreateBooking)
	booking.Post("/:bookingId/review", handlers.CreateReview) 
	booking.Post("/:bookingId/request-refund", handlers.RequestRefund) 
	booking.Post("/:bookingId/request-reschedule", handlers.RequestReschedule)
	booking.Post("/:bookingId/report-no-show", handlers.ReportNoShow)

	teacherBooking := api.Group("/teacher/bookings", middleware.Protected(), middleware.TeacherRequired())
	teacherBooking.Post("/:bookingId/complete", handlers.MarkBookingAsComplete)
	teacherBooking.Post("/:bookingId/feedback", handlers.SubmitTeacherFeedback)
	teacherBooking.Post("/:bookingId/rate-student", handlers.SubmitStudentRating)
	teacherBooking.Get("/pending-ratings", handlers.GetPendingStudentRatings)
}