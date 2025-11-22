package handlers

import (
	"github.com/anjiri1684/language_tutor/services"
	"github.com/gofiber/fiber/v2"
)

func GetConversionRate(c *fiber.Ctx) error {
	rates, err := services.FetchRates()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not fetch exchange rates"})
	}

	kesRate, ok := rates["KES"]
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "KES rate not available"})
	}

	return c.JSON(fiber.Map{"usd_to_kes": kesRate})
}