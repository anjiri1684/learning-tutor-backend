package routes

import (
    "github.com/anjiri1684/language_tutor/handlers"
    "github.com/anjiri1684/language_tutor/middleware"
    "github.com/gofiber/contrib/websocket"
    "github.com/gofiber/fiber/v2"
)

func MessagingRoutes(app *fiber.App) {
    api := app.Group("/api/v1")

    conversations := api.Group("/conversations", middleware.Protected())
    conversations.Get("", handlers.GetUserConversations)
    conversations.Post("", handlers.CreateOrGetConversation)
    conversations.Get("/:conversationId/messages", handlers.GetConversationMessages)

    api.Use("/ws", func(c *fiber.Ctx) error {
        if !websocket.IsWebSocketUpgrade(c) {
            return fiber.ErrUpgradeRequired
        }
        return c.Next()
    })
    api.Get("/ws", websocket.New(handlers.ServeWs))
}