package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func GamificationRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	gamification := api.Group("/gamification")
	gamification.Get("/leaderboard", handlers.GetLeaderboard)

	adminGamification := api.Group("/admin/gamification", middleware.Protected(), middleware.AdminRequired())

	badges := adminGamification.Group("/badges")
	badges.Post("", handlers.CreateBadge)
	badges.Get("", handlers.ListBadges)
	badges.Put("/:badgeId", handlers.UpdateBadge)
	badges.Delete("/:badgeId", handlers.DeleteBadge)

	userGamification := api.Group("/gamification", middleware.Protected())
	userGamification.Get("/certificates/me", handlers.ListMyCertificates)
	userGamification.Get("/badges/me", handlers.GetMyBadges)
}


	
	