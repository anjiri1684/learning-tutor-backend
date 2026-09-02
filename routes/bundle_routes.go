package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func BundleRoutes(app *fiber.App) {
	api := app.Group("/api/v1")
	api.Get("/bundles", handlers.ListActiveBundles)
	api.Get("/corporate-trainings", handlers.ListCorporateTrainings)

	// Per-route Protected() rather than a Group so the auth middleware does
	// not leak onto the public GET /bundles above.
	studentBundles := api.Group("/bundles")
	studentBundles.Get("/me", middleware.Protected(), handlers.GetMyBundles)
	studentBundles.Post("/:bundleId/purchase", middleware.Protected(), handlers.PurchaseBundle)
}
