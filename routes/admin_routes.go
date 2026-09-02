package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func AdminRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	admin := api.Group("/admin", middleware.Protected(), middleware.AdminAreaRequired())

	admin.Get("/applications/pending", middleware.AdminSection("teacher-applications"), handlers.ListPendingApplications)
	admin.Put("/applications/:teacherId", middleware.AdminSection("teacher-applications"), handlers.ManageApplication)
	admin.Post("/bookings/:bookingId/add-link", middleware.AdminSection("bookings"), handlers.AddMeetingLink)
	admin.Get("/dashboard-analytics", handlers.GetDashboardAnalytics)

	admin.Get("/refund-requests", middleware.AdminSection("refunds"), handlers.ListRefundRequests)
	admin.Post("/refund-requests/:paymentId/process", middleware.AdminSection("refunds"), handlers.ProcessRefund)

	reports := admin.Group("/reports", middleware.AdminSection("reports"))
	reports.Get("/transactions", handlers.GenerateTransactionReport)

	languages := admin.Group("/languages", middleware.AdminSection("languages"))
	languages.Post("", handlers.CreateLanguage)
	languages.Get("", handlers.ListLanguages)
	languages.Put("/:languageId", handlers.UpdateLanguage)
	languages.Delete("/:languageId", handlers.DeleteLanguage)

	users := admin.Group("/users", middleware.AdminSection("users"))
	users.Get("", handlers.GetAllUsers)
	users.Post("", handlers.AdminCreateUser)
	users.Post("/bulk-import", handlers.AdminBulkImportUsers)
	users.Put("/:userId", handlers.AdminUpdateUser)
	users.Put("/:userId/status", handlers.ToggleUserStatus)
	users.Delete("/:userId", handlers.AdminDeleteUser)
	users.Post("/:userId/impersonate", handlers.AdminImpersonateUser)

	teachers := admin.Group("/teachers", middleware.AdminSection("users"))
	teachers.Put("/:teacherId", handlers.AdminUpdateTeacherProfile)
	teachers.Put("/:teacherId/meeting-link", handlers.AdminUpdateTeacherMeetingLink)

	admin.Get("/payout-requests", middleware.AdminSection("payouts"), handlers.ListPayoutRequests)
	admin.Post("/payout-requests/:requestId/process", middleware.AdminSection("payouts"), handlers.ProcessPayoutRequest)

	admin.Get("/bookings", middleware.AdminSection("bookings"), handlers.AdminGetAllBookings)
	admin.Post("/bookings/assign", middleware.AdminSection("bookings"), handlers.AdminAssignClass)
	admin.Get("/audit-logs", middleware.AdminSection("audit-log"), handlers.ListAuditLogs)
	admin.Get("/payments", middleware.AdminSection("payments"), handlers.AdminGetPayments)
	admin.Post("/send-email", middleware.AdminSection("users"), handlers.AdminSendEmail)

	reviews := admin.Group("/reviews", middleware.AdminSection("reviews"))
	reviews.Get("", handlers.AdminGetReviews)
	reviews.Delete("/:reviewId", handlers.AdminDeleteReview)

	bundles := admin.Group("/bundles", middleware.AdminSection("bundles"))
	bundles.Get("", handlers.AdminListBundles)
	bundles.Post("", handlers.AdminCreateBundle)
	bundles.Put("/:bundleId", handlers.AdminUpdateBundle)
	bundles.Put("/:bundleId/status", handlers.ToggleBundleStatus)
	bundles.Post("/:bundleId/deactivate", handlers.AdminDeactivateBundle)
	bundles.Delete("/:bundleId", handlers.AdminDeleteBundle)

	corporateEnquiries := admin.Group("/corporate-enquiries", middleware.AdminSection("corporate-enquiries"))
	corporateEnquiries.Get("", handlers.AdminListCorporateEnquiries)
	corporateEnquiries.Put("/:enquiryId/status", handlers.AdminUpdateCorporateEnquiryStatus)

	contactRequests := admin.Group("/contact-requests", middleware.AdminSection("requests"))
	contactRequests.Get("", handlers.AdminListContactRequests)
	contactRequests.Put("/:requestId/status", handlers.AdminUpdateContactRequestStatus)
}
