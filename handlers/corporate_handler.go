package handlers

import (
	"strconv"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type CorporateEnquiryRequest struct {
	CompanyName string `json:"company_name" validate:"required,min=2"`
	ContactName string `json:"contact_name" validate:"required,min=2"`
	Email       string `json:"email" validate:"required,email"`
	Phone       string `json:"phone"`
	TeamSize    int    `json:"team_size" validate:"omitempty,gte=0"`
	LanguageID  string `json:"language_id" validate:"omitempty,uuid"`
	Message     string `json:"message"`
}

// CreateCorporateEnquiry captures a "request a custom quote" lead (public).
func CreateCorporateEnquiry(c *fiber.Ctx) error {
	var req CorporateEnquiryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	enquiry := models.CorporateEnquiry{
		CompanyName: req.CompanyName,
		ContactName: req.ContactName,
		Email:       req.Email,
		Phone:       req.Phone,
		TeamSize:    req.TeamSize,
		Message:     req.Message,
		Status:      "new",
	}
	if req.LanguageID != "" {
		if id, err := uuid.Parse(req.LanguageID); err == nil {
			enquiry.LanguageID = &id
		}
	}

	if err := database.DB.Create(&enquiry).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to submit enquiry"})
	}

	go services.NotifyAndEmailAllAdmins("corporate_enquiry", "New corporate training enquiry",
		req.CompanyName+" ("+req.ContactName+") requested a corporate training quote.", "/admin/corporate-enquiries",
		[]string{
			"Company: " + req.CompanyName,
			"Contact: " + req.ContactName,
			"Email: " + req.Email,
			"Phone: " + req.Phone,
			"Team size: " + strconv.Itoa(req.TeamSize),
			"Message: " + req.Message,
		})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Thank you. Our team will be in touch shortly.",
		"enquiry": enquiry,
	})
}

// AdminListCorporateEnquiries lists all corporate training enquiries.
func AdminListCorporateEnquiries(c *fiber.Ctx) error {
	var enquiries []models.CorporateEnquiry
	database.DB.Preload("Language").Order("created_at desc").Find(&enquiries)
	return c.JSON(enquiries)
}

type UpdateEnquiryStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=new contacted closed"`
}

// AdminUpdateCorporateEnquiryStatus updates the follow-up status of an enquiry.
func AdminUpdateCorporateEnquiryStatus(c *fiber.Ctx) error {
	enquiryID := c.Params("enquiryId")
	var req UpdateEnquiryStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	result := database.DB.Model(&models.CorporateEnquiry{}).Where("id = ?", enquiryID).Update("status", req.Status)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update enquiry"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Enquiry not found"})
	}
	return c.JSON(fiber.Map{"message": "Enquiry status updated"})
}
