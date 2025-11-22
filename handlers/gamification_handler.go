package handlers

import (
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type BadgeRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description" validate:"required"`
	IconURL     string `json:"icon_url" validate:"required,url"`
}

func CreateBadge(c *fiber.Ctx) error {
	var req BadgeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	badge := models.Badge{
		Name:        req.Name,
		Description: req.Description,
		IconURL:     req.IconURL,
	}

	if err := database.DB.Create(&badge).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create badge"})
	}

	return c.Status(fiber.StatusCreated).JSON(badge)
}

func ListBadges(c *fiber.Ctx) error {
	var badges []models.Badge
	database.DB.Find(&badges)
	return c.JSON(badges)
}

func UpdateBadge(c *fiber.Ctx) error {
	badgeID := c.Params("badgeId")
	var badge models.Badge
	if err := database.DB.First(&badge, "id = ?", badgeID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Badge not found"})
	}

	var req BadgeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	badge.Name = req.Name
	badge.Description = req.Description
	badge.IconURL = req.IconURL
	database.DB.Save(&badge)

	return c.JSON(badge)
}

func DeleteBadge(c *fiber.Ctx) error {
	badgeID := c.Params("badgeId")
	result := database.DB.Delete(&models.Badge{}, "id = ?", badgeID)

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete badge"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Badge not found"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}



type LeaderboardUser struct {
	FullName          string  `json:"full_name"`
	XP                int     `json:"xp"`
	ProfilePictureURL *string `json:"profile_picture_url"`
}

func GetLeaderboard(c *fiber.Ctx) error {
	var leaderboard []LeaderboardUser

	err := database.DB.Model(&models.User{}).
		Select("full_name", "xp", "profile_picture_url").
		Order("xp desc").
		Limit(10).
		Find(&leaderboard).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve leaderboard"})
	}

	return c.JSON(leaderboard)
}


func ListMyCertificates(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))
	
	var certificates []models.Certificate
	database.DB.Where("student_id = ?", studentID).Find(&certificates)

	return c.JSON(certificates)
}


func GetMyBadges(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))

	var user models.User
	if err := database.DB.Preload("Badges").First(&user, "id = ?", studentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(user.Badges)
}