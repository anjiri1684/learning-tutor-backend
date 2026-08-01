package handlers

import (
	"strconv"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

func ListMyNotifications(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))
	offset := (page - 1) * pageSize

	var notifications []models.Notification
	database.DB.
		Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(pageSize).
		Offset(offset).
		Find(&notifications)

	var unreadCount int64
	database.DB.Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Count(&unreadCount)

	return c.JSON(fiber.Map{
		"data":         notifications,
		"unread_count": unreadCount,
	})
}

func MarkNotificationRead(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	notificationID := c.Params("notificationId")

	now := time.Now()
	result := database.DB.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Update("read_at", now)

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Notification not found"})
	}

	return c.JSON(fiber.Map{"message": "Notification marked as read"})
}

func MarkAllNotificationsRead(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))

	now := time.Now()
	database.DB.Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", now)

	return c.JSON(fiber.Map{"message": "All notifications marked as read"})
}
