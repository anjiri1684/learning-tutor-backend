package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func ExamRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	exam := api.Group("/admin/exams", middleware.Protected(), middleware.AdminRequired())

	questions := exam.Group("/questions")
	questions.Post("", handlers.CreateQuestion)
	questions.Get("", handlers.ListQuestions)
	questions.Get("/:questionId", handlers.GetQuestion)
	questions.Put("/:questionId", handlers.UpdateQuestion)
	questions.Delete("/:questionId", handlers.DeleteQuestion)
	
	tests := exam.Group("/tests")
	tests.Post("", handlers.CreateMockTest)
	tests.Get("", handlers.ListMockTests)
	tests.Get("/:testId", handlers.GetMockTest)
	tests.Put("/:testId", handlers.UpdateMockTest)
	tests.Delete("/:testId", handlers.DeleteMockTest)

	studentExams := api.Group("/exams", middleware.Protected())
	studentExams.Get("/tests", handlers.StudentListMockTests)
	studentExams.Post("/tests/:testId/start", handlers.StartTestAttempt)
	studentExams.Post("/tests/submit/:attemptId", handlers.SubmitTestAttempt)
}