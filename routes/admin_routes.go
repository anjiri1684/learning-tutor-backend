package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func AdminRoutes(app *fiber.App) {
	api := app.Group("/api/v1")
	
	admin := api.Group("/admin", middleware.Protected(), middleware.AdminRequired())

	admin.Get("/applications/pending", handlers.ListPendingApplications)
	admin.Put("/applications/:teacherId", handlers.ManageApplication)
	admin.Post("/bookings/:bookingId/add-link", handlers.AddMeetingLink)
	admin.Get("/dashboard-analytics", handlers.GetDashboardAnalytics)

	admin.Get("/refund-requests", handlers.ListRefundRequests)
	admin.Post("/refund-requests/:paymentId/process", handlers.ProcessRefund)

	reports := admin.Group("/reports")
	reports.Get("/transactions", handlers.GenerateTransactionReport)

	languages := admin.Group("/languages")
	languages.Post("", handlers.CreateLanguage)
	languages.Get("", handlers.ListLanguages)
	languages.Put("/:languageId", handlers.UpdateLanguage)
	languages.Delete("/:languageId", handlers.DeleteLanguage)

	users := admin.Group("/users")
	users.Get("", handlers.GetAllUsers)
	users.Put("/:userId/status", handlers.ToggleUserStatus)
	users.Delete("/:userId", handlers.AdminDeleteUser) 


	admin.Get("/payout-requests", handlers.ListPayoutRequests)
	admin.Post("/payout-requests/:requestId/process", handlers.ProcessPayoutRequest)

	admin.Get("/bookings", handlers.AdminGetAllBookings)
	admin.Get("/payments", handlers.AdminGetPayments)
	

	reviews := admin.Group("/reviews")
	reviews.Get("", handlers.AdminGetReviews)
	reviews.Delete("/:reviewId", handlers.AdminDeleteReview)

	bundles := admin.Group("/bundles")
	bundles.Get("", handlers.AdminListBundles)
	bundles.Post("", handlers.AdminCreateBundle)
	bundles.Put("/:bundleId", handlers.AdminUpdateBundle)
	bundles.Delete("/:bundleId", handlers.AdminDeactivateBundle)

}
