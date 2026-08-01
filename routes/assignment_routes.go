package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func AssignmentRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	protected := api.Group("", middleware.Protected())

	protected.Get("/assignments/me", handlers.ListMyAssignments)
	protected.Get("/bookings/:bookingId/assignments", handlers.ListBookingAssignments)

	teacherBooking := protected.Group("/teacher/bookings/:bookingId/assignments", middleware.TeacherRequired())
	teacherBooking.Post("", handlers.CreateAssignment)

	protected.Post("/assignments/:assignmentId/submit", handlers.SubmitAssignment)
	protected.Get("/assignments/:assignmentId/submission/file", handlers.ViewAssignmentSubmissionFile)

	teacherAssignment := protected.Group("/teacher/assignments/:assignmentId", middleware.TeacherRequired())
	teacherAssignment.Post("/grade", handlers.GradeAssignment)
}
