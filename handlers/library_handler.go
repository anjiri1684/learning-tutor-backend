package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

const maxLibraryResourceFileSize = 20 * 1024 * 1024 // 20MB

func claimsUserAndRole(c *fiber.Ctx) (uuid.UUID, string) {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	role := claims["role"].(string)
	return userID, role
}

func UploadLibraryResource(c *fiber.Ctx) error {
	userID, _ := claimsUserAndRole(c)

	title := c.FormValue("title")
	description := c.FormValue("description")
	if title == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Title is required"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File is required"})
	}
	if file.Size > maxLibraryResourceFileSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File is too large (max 20MB)"})
	}

	fileHandle, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open uploaded file"})
	}
	defer fileHandle.Close()

	resourceID := uuid.New()
	objectKey := fmt.Sprintf("library/%s/%s", resourceID.String(), file.Filename)

	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := services.UploadObject(objectKey, fileHandle, contentType); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to upload file to storage"})
	}

	resource := models.LibraryResource{
		ID:          resourceID,
		UploadedBy:  userID,
		Title:       title,
		Description: description,
		ObjectKey:   objectKey,
		FileName:    file.Filename,
	}
	if err := database.DB.Create(&resource).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save resource record"})
	}

	return c.Status(fiber.StatusCreated).JSON(resource)
}

func ListMyLibraryResources(c *fiber.Ctx) error {
	userID, role := claimsUserAndRole(c)

	query := database.DB.Preload("Access").Preload("Uploader").Order("created_at desc")
	if role != "admin" {
		query = query.Where("uploaded_by = ?", userID)
	}

	var resources []models.LibraryResource
	query.Find(&resources)

	return c.JSON(resources)
}

func ListAccessibleLibraryResources(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	var resources []models.LibraryResource
	database.DB.
		Joins("JOIN library_resource_accesses ON library_resource_accesses.library_resource_id = library_resources.id").
		Where("LOWER(library_resource_accesses.student_email) = LOWER(?) AND library_resource_accesses.revoked_at IS NULL", user.Email).
		Preload("Uploader").
		Order("library_resources.created_at desc").
		Find(&resources)

	return c.JSON(resources)
}

func canManageLibraryResource(resource models.LibraryResource, userID uuid.UUID, role string) bool {
	return role == "admin" || resource.UploadedBy == userID
}

type GrantLibraryAccessRequest struct {
	Emails []string `json:"emails" validate:"required,min=1,dive,email"`
}

func GrantLibraryAccess(c *fiber.Ctx) error {
	userID, role := claimsUserAndRole(c)
	resourceID := c.Params("resourceId")

	var resource models.LibraryResource
	if err := database.DB.First(&resource, "id = ?", resourceID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Resource not found"})
	}

	if !canManageLibraryResource(resource, userID, role) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not own this resource"})
	}

	var req GrantLibraryAccessRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	type SkippedEmail struct {
		Email  string `json:"email"`
		Reason string `json:"reason"`
	}

	var granted []models.LibraryResourceAccess
	var skipped []SkippedEmail

	for _, rawEmail := range req.Emails {
		email := strings.ToLower(strings.TrimSpace(rawEmail))
		if email == "" {
			continue
		}

		var student models.User
		hasStudentAccount := database.DB.Where("LOWER(email) = ?", email).First(&student).Error == nil

		if role != "admin" {
			var activeBookingCount int64
			database.DB.Model(&models.Booking{}).
				Joins("JOIN users ON users.id = bookings.student_id").
				Where("bookings.teacher_id = ? AND LOWER(users.email) = ? AND bookings.status IN ('confirmed', 'completed')", resource.UploadedBy, email).
				Count(&activeBookingCount)

			if activeBookingCount == 0 {
				skipped = append(skipped, SkippedEmail{Email: email, Reason: "No active or completed class with this student"})
				continue
			}
		}

		var existingAccess models.LibraryResourceAccess
		err := database.DB.Where("library_resource_id = ? AND LOWER(student_email) = ?", resource.ID, email).First(&existingAccess).Error

		if err == nil {
			existingAccess.RevokedAt = nil
			existingAccess.GrantedAt = time.Now()
			database.DB.Save(&existingAccess)
			granted = append(granted, existingAccess)
			continue
		}

		access := models.LibraryResourceAccess{
			LibraryResourceID: resource.ID,
			StudentEmail:      email,
			GrantedAt:         time.Now(),
		}
		if hasStudentAccount {
			access.StudentID = &student.ID
		}
		if err := database.DB.Create(&access).Error; err == nil {
			granted = append(granted, access)
			if hasStudentAccount {
				go services.CreateNotification(student.ID, "resource_shared", "New resource shared",
					"\""+resource.Title+"\" has been shared with you.", "/dashboard/my-library")
			}
		} else {
			skipped = append(skipped, SkippedEmail{Email: email, Reason: "Failed to save access grant"})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"granted": granted,
		"skipped": skipped,
	})
}

func RevokeLibraryAccess(c *fiber.Ctx) error {
	userID, role := claimsUserAndRole(c)
	resourceID := c.Params("resourceId")
	email := c.Params("email")

	var resource models.LibraryResource
	if err := database.DB.First(&resource, "id = ?", resourceID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Resource not found"})
	}

	if !canManageLibraryResource(resource, userID, role) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not own this resource"})
	}

	now := time.Now()
	result := database.DB.Model(&models.LibraryResourceAccess{}).
		Where("library_resource_id = ? AND LOWER(student_email) = LOWER(?)", resourceID, email).
		Update("revoked_at", now)

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Access grant not found"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func GetResourceNote(c *fiber.Ctx) error {
	userID, _ := claimsUserAndRole(c)
	resourceID := c.Params("resourceId")

	var note models.ResourceNote
	if err := database.DB.Where("user_id = ? AND library_resource_id = ?", userID, resourceID).First(&note).Error; err != nil {
		return c.JSON(fiber.Map{"note": ""})
	}

	return c.JSON(fiber.Map{"note": note.Note})
}

type SaveResourceNoteRequest struct {
	Note string `json:"note"`
}

func SaveResourceNote(c *fiber.Ctx) error {
	userID, _ := claimsUserAndRole(c)
	resourceID, err := uuid.Parse(c.Params("resourceId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid resource ID"})
	}

	var req SaveResourceNoteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	var note models.ResourceNote
	err = database.DB.Where("user_id = ? AND library_resource_id = ?", userID, resourceID).First(&note).Error
	if err == nil {
		note.Note = req.Note
		database.DB.Save(&note)
	} else {
		note = models.ResourceNote{
			UserID:            userID,
			LibraryResourceID: resourceID,
			Note:              req.Note,
		}
		database.DB.Create(&note)
	}

	return c.JSON(fiber.Map{"note": note.Note})
}

func DeleteLibraryResource(c *fiber.Ctx) error {
	userID, role := claimsUserAndRole(c)
	resourceID := c.Params("resourceId")

	var resource models.LibraryResource
	if err := database.DB.First(&resource, "id = ?", resourceID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Resource not found"})
	}

	if !canManageLibraryResource(resource, userID, role) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not own this resource"})
	}

	if err := database.DB.Where("library_resource_id = ?", resource.ID).Delete(&models.LibraryResourceAccess{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to remove access grants"})
	}

	if err := database.DB.Delete(&resource).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete resource"})
	}

	go services.DeleteObject(resource.ObjectKey)

	return c.SendStatus(fiber.StatusNoContent)
}

func ViewLibraryResource(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	role := claims["role"].(string)
	resourceID := c.Params("resourceId")

	var resource models.LibraryResource
	if err := database.DB.First(&resource, "id = ?", resourceID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Resource not found"})
	}

	hasAccess := role == "admin" || resource.UploadedBy == userID

	if !hasAccess {
		var user models.User
		if err := database.DB.First(&user, "id = ?", userID).Error; err == nil {
			var accessCount int64
			database.DB.Model(&models.LibraryResourceAccess{}).
				Where("library_resource_id = ? AND LOWER(student_email) = LOWER(?) AND revoked_at IS NULL", resourceID, user.Email).
				Count(&accessCount)
			hasAccess = accessCount > 0
		}
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not have access to this resource"})
	}

	stream, contentType, err := services.GetObjectStream(resource.ObjectKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve file from storage"})
	}
	
	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", resource.FileName))
	c.Set("Cache-Control", "no-store")

	return c.SendStream(stream)
}
