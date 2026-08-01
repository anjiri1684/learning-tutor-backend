package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func NotificationRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	notifications := api.Group("/notifications", middleware.Protected())
	notifications.Get("", handlers.ListMyNotifications)
	notifications.Put("/read-all", handlers.MarkAllNotificationsRead)
	notifications.Put("/:notificationId/read", handlers.MarkNotificationRead)
}
