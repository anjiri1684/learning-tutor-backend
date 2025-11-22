package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func BundleRoutes(app *fiber.App) {
	api := app.Group("/api/v1")
	api.Get("/bundles", handlers.ListActiveBundles)
	
	studentBundles := api.Group("/bundles", middleware.Protected())
	studentBundles.Get("/me", handlers.GetMyBundles)
	studentBundles.Post("/:bundleId/purchase", handlers.PurchaseBundle)


	adminBundles := api.Group("/admin/bundles", middleware.Protected(), middleware.AdminRequired())
	adminBundles.Get("", handlers.AdminListBundles) 
	adminBundles.Post("", handlers.CreateBundle)
	adminBundles.Put("/:bundleId", handlers.UpdateBundle)
	adminBundles.Put("/:bundleId/status", handlers.ToggleBundleStatus) 
}