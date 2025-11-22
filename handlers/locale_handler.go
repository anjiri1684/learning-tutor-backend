package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func GetLocale(c *fiber.Ctx) error {
	lang := c.Params("lang")

	cleanLang := filepath.Clean(filepath.Base(lang))
	if strings.Contains(cleanLang, "..") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid language parameter"})
	}

	filePath := filepath.Join("locales", fmt.Sprintf("%s.json", cleanLang))

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Language file not found"})
	}

	return c.SendFile(filePath)
}