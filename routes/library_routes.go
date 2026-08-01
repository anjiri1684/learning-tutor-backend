package routes

import (
	"github.com/anjiri1684/language_tutor/handlers"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/gofiber/fiber/v2"
)

func LibraryRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	library := api.Group("/library", middleware.Protected())

	library.Get("/me", handlers.ListAccessibleLibraryResources)
	library.Get("/:resourceId/view", handlers.ViewLibraryResource)
	library.Get("/:resourceId/note", handlers.GetResourceNote)
	library.Put("/:resourceId/note", handlers.SaveResourceNote)

	managed := library.Group("", middleware.TeacherOrAdminRequired())
	managed.Post("", handlers.UploadLibraryResource)
	managed.Get("/mine", handlers.ListMyLibraryResources)
	managed.Post("/:resourceId/access", handlers.GrantLibraryAccess)
	managed.Delete("/:resourceId/access/:email", handlers.RevokeLibraryAccess)
	managed.Delete("/:resourceId", handlers.DeleteLibraryResource)
}
