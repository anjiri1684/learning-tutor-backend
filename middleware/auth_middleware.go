package middleware

import (
	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"
	"github.com/golang-jwt/jwt/v4"
)

func Protected() fiber.Handler {
	return jwtware.New(jwtware.Config{
		SigningKey:   []byte(config.Config("JWT_SECRET")),
		ErrorHandler: jwtError,
	})
}

func jwtError(c *fiber.Ctx, err error) error {
	if err.Error() == "Missing or malformed JWT" {
		return c.Status(fiber.StatusBadRequest).
			JSON(fiber.Map{"status": "error", "message": "Missing or malformed JWT", "data": nil})
	}
	return c.Status(fiber.StatusUnauthorized).
		JSON(fiber.Map{"status": "error", "message": "Invalid or expired JWT", "data": nil})
}


func AdminRequired() fiber.Handler {
    return func(c *fiber.Ctx) error {
        token := c.Locals("user").(*jwt.Token)
        claims := token.Claims.(jwt.MapClaims)
        role := claims["role"].(string)

        if role != "admin" {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": "Forbidden: Admin access required",
            })
        }
        return c.Next()
    }
}

func TeacherRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Locals("user").(*jwt.Token)
		claims := token.Claims.(jwt.MapClaims)
		role := claims["role"].(string)

		if role != "teacher" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Forbidden: Teacher access required",
			})
		}
		return c.Next()
	}
}