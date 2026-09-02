package handlers

import (
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/gofiber/fiber/v2"
)

type ContactRequestInput struct {
	Name    string `json:"name" validate:"required,min=2"`
	Email   string `json:"email" validate:"required,email"`
	Subject string `json:"subject"`
	Message string `json:"message" validate:"required,min=5"`
	Type    string `json:"type" validate:"omitempty,oneof=general teacher_application other"`
}

// CreateContactRequest stores a message from a public contact form (replaces mailto:).
func CreateContactRequest(c *fiber.Ctx) error {
	var req ContactRequestInput
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	reqType := req.Type
	if reqType == "" {
		reqType = "general"
	}

	contact := models.ContactRequest{
		Name:    req.Name,
		Email:   req.Email,
		Subject: req.Subject,
		Message: req.Message,
		Type:    reqType,
		Status:  "new",
	}
	if err := database.DB.Create(&contact).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to submit your message"})
	}

	go services.NotifyAndEmailAllAdmins("contact_request", "New contact request",
		req.Name+" sent a message via the website.", "/admin/requests",
		[]string{
			"From: " + req.Name,
			"Email: " + req.Email,
			"Subject: " + req.Subject,
			"Type: " + reqType,
			"Message: " + req.Message,
		})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Thank you. Your message has been sent and our team will get back to you.",
	})
}

// AdminListContactRequests lists contact requests, newest first, optionally filtered by status.
func AdminListContactRequests(c *fiber.Ctx) error {
	var requests []models.ContactRequest
	query := database.DB.Order("created_at desc")
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	query.Find(&requests)

	var newCount int64
	database.DB.Model(&models.ContactRequest{}).Where("status = ?", "new").Count(&newCount)

	return c.JSON(fiber.Map{"data": requests, "new_count": newCount})
}

type UpdateContactRequestStatusInput struct {
	Status string `json:"status" validate:"required,oneof=new read resolved"`
}

// AdminUpdateContactRequestStatus updates the review status of a contact request.
func AdminUpdateContactRequestStatus(c *fiber.Ctx) error {
	id := c.Params("requestId")
	var req UpdateContactRequestStatusInput
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	result := database.DB.Model(&models.ContactRequest{}).Where("id = ?", id).Update("status", req.Status)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update request"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Request not found"})
	}
	return c.JSON(fiber.Map{"message": "Request updated"})
}
