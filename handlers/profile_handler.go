package handlers

import (
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type UpdateProfileRequest struct {
	FullName         *string `json:"full_name"`
	ProfilePictureURL *string `json:"profile_picture_url"`
	TimeZone         *string `json:"time_zone"`
	LearningGoals    *string `json:"learning_goals"`
	ProficiencyLevel *string `json:"proficiency_level"`
}


func GetProfile(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID := claims["user_id"].(string)

	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(user)
}

func UpdateProfile(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID := claims["user_id"].(string)

	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	if req.FullName != nil {
		user.FullName = *req.FullName
	}
	if req.ProfilePictureURL != nil {
		user.ProfilePictureURL = req.ProfilePictureURL
	}
	if req.TimeZone != nil {
		user.TimeZone = req.TimeZone
	}
	if req.LearningGoals != nil {
		user.LearningGoals = req.LearningGoals
	}
	if req.ProficiencyLevel != nil {
		user.ProficiencyLevel = req.ProficiencyLevel
	}

	database.DB.Save(&user)
	
	return c.JSON(user)
}


func GetMyProgress(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))

	var totalClasses int64
	database.DB.Model(&models.Booking{}).
		Where("student_id = ? AND status = ?", studentID, "completed").
		Count(&totalClasses)

	var totalHours float64
	database.DB.Model(&models.Booking{}).
		Joins("JOIN availability_slots on bookings.availability_slot_id = availability_slots.id").
		Where("bookings.student_id = ? AND bookings.status = ?", studentID, "completed").
		Select("COALESCE(SUM(EXTRACT(EPOCH FROM (availability_slots.end_time - availability_slots.start_time))) / 3600, 0)").
		Row().Scan(&totalHours)

	var testHistory []models.TestAttempt
	database.DB.Preload("MockTest").
		Where("student_id = ?", studentID).
		Order("start_time desc").
		Find(&testHistory)

	return c.JSON(fiber.Map{
		"total_classes_completed": totalClasses,
		"total_hours_learned":     totalHours,
		"test_history":            testHistory,
	})
}