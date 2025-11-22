package handlers

import (
	"context"
	"fmt"
	"time"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

func UploadResource(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var booking models.Booking
	if err := database.DB.First(&booking, "id = ? AND teacher_id = ?", bookingID, teacherID).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Booking not found or you are not the teacher for this class."})
	}

	file, err := c.FormFile("resource")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Resource file is required."})
	}

	cld, _ := cloudinary.NewFromURL(config.Config("CLOUDINARY_URL"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uploadResult, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder: "language_tutor_resources",
		PublicID: fmt.Sprintf("booking_%s_%s", bookingID, file.Filename),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to upload file."})
	}

	resource := models.Resource{
		BookingID:  booking.ID,
		FileName:   file.Filename,
		FileURL:    uploadResult.SecureURL,
		UploadedAt: time.Now(),
	}
	database.DB.Create(&resource)

	return c.Status(fiber.StatusCreated).JSON(resource)
}

func GetBookingResources(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var booking models.Booking
	if err := database.DB.First(&booking, "id = ?", bookingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Booking not found"})
	}

	if booking.StudentID != userID && booking.TeacherID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not have access to this booking's resources."})
	}

	var resources []models.Resource
	database.DB.Where("booking_id = ?", bookingID).Find(&resources)

	return c.JSON(resources)
}