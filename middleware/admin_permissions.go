package middleware

import (
	"encoding/json"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

var AdminSections = []string{
	"dashboard",
	"users",
	"teacher-applications",
	"bookings",
	"languages",
	"bundles",
	"corporate-enquiries",
	"requests",
	"exams",
	"payouts",
	"refunds",
	"payments",
	"reviews",
	"reports",
	"library",
	"audit-log",
	// "settings" is intentionally not restrictable: it is each user's own
	// profile / password page, always available.
}

// CoachHasSection reports whether a coach's stored permissions blob grants
// access to slug. A nil / empty / malformed blob grants nothing.
func CoachHasSection(perms *string, slug string) bool {
	if perms == nil {
		return false
	}
	trimmed := *perms
	if trimmed == "" || trimmed == "[]" || trimmed == "null" {
		return false
	}
	var list []string
	if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
		return false
	}
	for _, s := range list {
		if s == slug {
			return true
		}
	}
	return false
}

// AdminAreaRequired allows full admins and coaches into the admin area.
// Must run after Protected().
func AdminAreaRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Locals("user").(*jwt.Token)
		claims := token.Claims.(jwt.MapClaims)
		role, _ := claims["role"].(string)
		if role != "admin" && role != "coach" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden: Admin access required"})
		}
		return c.Next()
	}
}

// AdminSection returns middleware that permits the request when the caller is a
// full admin, or a coach who has been granted `slug`.
// Must run after Protected() + AdminAreaRequired().
func AdminSection(slug string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Locals("user").(*jwt.Token)
		claims := token.Claims.(jwt.MapClaims)
		role, _ := claims["role"].(string)

		if role == "admin" {
			return c.Next()
		}
		if role != "coach" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden: Admin access required"})
		}

		userID, _ := claims["user_id"].(string)
		var user models.User
		if err := database.DB.Select("role", "admin_permissions").First(&user, "id = ?", userID).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
		}

		if !CoachHasSection(user.AdminPermissions, slug) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden: you do not have access to this section"})
		}
		return c.Next()
	}
}
