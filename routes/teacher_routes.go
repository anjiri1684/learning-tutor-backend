package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)



func TeacherRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	api.Get("/teachers", handlers.ListActiveTeachers)
	api.Get("/teachers/:teacherId/availability", handlers.GetTeacherAvailability)
	api.Get("/languages", handlers.ListLanguages) 
	api.Get("/teachers/:teacherId", handlers.GetTeacherProfile)


	teacher := api.Group("/teacher", middleware.Protected())
	teacher.Post("/apply", handlers.ApplyToBeATeacher)
	teacher.Get("/bookings", handlers.GetMyTeacherBookings)
	teacher.Get("/earnings", handlers.GetTeacherEarnings) 
	teacher.Get("/reviews/me", handlers.GetMyReviews) 
	teacher.Get("/student-progress/:studentId", handlers.GetStudentProgressForTeacher) 
	teacher.Get("/analytics", handlers.GetTeacherAnalytics) 
	
	availability := teacher.Group("/availability", middleware.TeacherRequired())
	availability.Post("", handlers.CreateAvailabilitySlot)
	availability.Get("/me", handlers.GetMyAvailability)
	availability.Delete("/:slotId", handlers.DeleteAvailabilitySlot) 

	profile := teacher.Group("/profile")
	profile.Get("/me", handlers.GetMyTeacherProfile)
	profile.Put("/me", handlers.UpdateMyTeacherProfile)
	

	teacherLanguages := teacher.Group("/languages", middleware.TeacherRequired())
	teacherLanguages.Post("", handlers.AddLanguageToProfile)
	teacherLanguages.Delete("/:languageId", handlers.RemoveLanguageFromProfile)

	reschedule := teacher.Group("/reschedule-requests")
	reschedule.Get("", handlers.ListRescheduleRequests)
	reschedule.Post("/:bookingId/process", handlers.ProcessReschedule)

	payouts := teacher.Group("/payouts", middleware.TeacherRequired())
	payouts.Post("/request", handlers.RequestPayout)
	payouts.Get("/requests", handlers.GetMyPayoutRequests) 

}