package handlers

import (
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

func JoinWaitlist(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))
	slotID := c.Params("slotId")

	var slot models.AvailabilitySlot
	if err := database.DB.First(&slot, "id = ?", slotID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Availability slot not found"})
	}

	if slot.Status == "available" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "This slot still has open spots, you can book it directly."})
	}

	var existing models.Waitlist
	if err := database.DB.Where("availability_slot_id = ? AND student_id = ?", slotID, studentID).First(&existing).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "You are already on the waitlist for this class."})
	}

	entry := models.Waitlist{
		AvailabilitySlotID: slot.ID,
		StudentID:          studentID,
	}
	if err := database.DB.Create(&entry).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to join waitlist"})
	}

	return c.Status(fiber.StatusCreated).JSON(entry)
}

func LeaveWaitlist(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))
	slotID := c.Params("slotId")

	result := database.DB.Where("availability_slot_id = ? AND student_id = ?", slotID, studentID).Delete(&models.Waitlist{})
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "You are not on the waitlist for this class."})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func ListMyWaitlist(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))

	var entries []models.Waitlist
	database.DB.
		Preload("AvailabilitySlot.Language").
		Preload("AvailabilitySlot.Teacher").
		Where("student_id = ?", studentID).
		Order("created_at desc").
		Find(&entries)

	return c.JSON(entries)
}
