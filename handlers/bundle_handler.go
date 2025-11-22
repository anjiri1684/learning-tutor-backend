package handlers

import (
	"log"
	"math"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/anjiri1684/language_tutor/payments"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BundleRequest struct {
	Name            string  `json:"name" validate:"required"`
	LanguageID      string  `json:"language_id" validate:"required,uuid"`
	NumberOfClasses int     `json:"number_of_classes" validate:"required,gt=0"`
	Price           float64 `json:"price" validate:"required,gt=0"`
}


func CreateBundle(c *fiber.Ctx) error {
	var req BundleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	bundle := models.Bundle{
		Name:            req.Name,
		LanguageID:      uuid.MustParse(req.LanguageID),
		NumberOfClasses: req.NumberOfClasses,
		Price:           req.Price,
	}

	if err := database.DB.Create(&bundle).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create bundle"})
	}
	return c.Status(fiber.StatusCreated).JSON(bundle)
}

func UpdateBundle(c *fiber.Ctx) error {
	bundleID := c.Params("bundleId")
	var bundle models.Bundle
	if err := database.DB.First(&bundle, "id = ?", bundleID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Bundle not found"})
	}

	var req BundleRequest
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

	bundle.Name = req.Name
	bundle.LanguageID = uuid.MustParse(req.LanguageID)
	bundle.NumberOfClasses = req.NumberOfClasses
	bundle.Price = req.Price
	database.DB.Save(&bundle)

	return c.JSON(bundle)
}

func DeactivateBundle(c *fiber.Ctx) error {
	bundleID := c.Params("bundleId")
	result := database.DB.Model(&models.Bundle{}).Where("id = ?", bundleID).Update("is_active", false)

	if result.Error != nil { return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to deactivate bundle"}) }
	if result.RowsAffected == 0 { return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Bundle not found"}) }
	
	return c.SendStatus(fiber.StatusNoContent)
}


func ListActiveBundles(c *fiber.Ctx) error {
	var bundles []models.Bundle
	database.DB.Preload("Language").Where("is_active = ?", true).Find(&bundles)
	return c.JSON(bundles)
}


type PurchaseBundleRequest struct {
	UseCredit        bool   `json:"use_credit"`
	PaymentProvider  string `json:"payment_provider,omitempty"`
	MpesaPhoneNumber string `json:"mpesa_phone_number,omitempty"`
}

func PurchaseBundle(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))
	bundleID, err := uuid.Parse(c.Params("bundleId"))
	if err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid bundle ID format"}) }

	var req PurchaseBundleRequest
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }
	
	var bundle models.Bundle
	if err := database.DB.First(&bundle, "id = ? AND is_active = ?", bundleID, true).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Active bundle not found"})
	}

	if req.UseCredit {
		var student models.User
		if err := database.DB.First(&student, "id = ?", studentID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Student not found"})
		}
		
		if student.CreditBalance >= bundle.Price {
			var activeBundle models.StudentBundle
			err := database.DB.Transaction(func(tx *gorm.DB) error {
				student.CreditBalance -= bundle.Price
				if err := tx.Save(&student).Error; err != nil { return err }

				activeBundle = models.StudentBundle{
					StudentID: studentID, BundleID: bundle.ID, PurchaseDate: time.Now(),
					RemainingClasses: bundle.NumberOfClasses, Status: "active",
				}
				if err := tx.Create(&activeBundle).Error; err != nil { return err }
				
				payment := models.Payment{
					StudentBundleID: &activeBundle.ID, 
					Amount:          bundle.Price, 
					Currency:        bundle.Currency, 
					Provider:        "credit", 
					Status:          "succeeded",
				}
				if err := tx.Create(&payment).Error; err != nil { return err }
				
				go notifications.SendEmail(student.FullName, student.Email, "Bundle Purchase Confirmed!", "<h1>Success!</h1><p>Your class bundle has been purchased with your credit balance and is now active.</p>")
				return nil
			})
			if err != nil { return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process credit payment for bundle: " + err.Error()}) }
			
			return c.Status(fiber.StatusCreated).JSON(fiber.Map{
				"message": "Bundle purchased successfully using your credit balance.",
				"student_bundle": activeBundle,
			})
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Insufficient credit balance to purchase this bundle."})
		}
	}

	var price = bundle.Price
	var currency = bundle.Currency

	if req.PaymentProvider == "mpesa" {
		if currency != "KES" {
			kesPrice, err := services.ConvertUSDToKES(price)
			if err != nil {
				log.Printf("🔥 Currency conversion failed: %v", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not perform currency conversion."})
			}
			price = math.Round(kesPrice) 
			currency = "KES"
		}
	}

	var studentBundle models.StudentBundle
	var payment models.Payment

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		studentBundle = models.StudentBundle{
			StudentID:        studentID,
			BundleID:         bundle.ID,
			PurchaseDate:     time.Now(),
			RemainingClasses: bundle.NumberOfClasses,
			Status:           "pending_payment",
		}
		if err := tx.Create(&studentBundle).Error; err != nil { return err }

		payment = models.Payment{
			StudentBundleID: &studentBundle.ID,
			Amount:          price,
			Currency:        currency, 
			Provider:        req.PaymentProvider,
			Status:          "pending",
		}
		if err := tx.Create(&payment).Error; err != nil { return err }
		return nil
	})
	if err != nil { return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create purchase records"}) }

	if req.PaymentProvider == "mpesa" {
		if req.MpesaPhoneNumber == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "M-Pesa phone number is required"})
		}

		stkResponse, err := payments.InitiateMpesaSTKPush(price, req.MpesaPhoneNumber, payment.ID.String())
		if err != nil {
			log.Printf("🔥 CRITICAL: InitiateMpesaSTKPush failed: %v", err)
			if err.Error() == "invalid M-Pesa phone number format" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Payment could not be initiated, please try again."})
		}

		payment.MerchantRequestID = &stkResponse.Response.MerchantRequestID
		database.DB.Save(&payment) 

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"student_bundle":   studentBundle,
			"customer_message": stkResponse.Response.CustomerMessage,
		})
	}

	if req.PaymentProvider == "paypal" {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"student_bundle": studentBundle,
			"payment_id":     payment.ID,
		})
	}

	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payment provider"})
}



func GetMyBundles(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))

	var myBundles []models.StudentBundle
	
	database.DB.
		Preload("Bundle.Language").
		Where("student_id = ? AND status = ?", studentID, "active").
		Find(&myBundles)

	return c.JSON(myBundles)
}

func ToggleBundleStatus(c *fiber.Ctx) error {
	bundleID := c.Params("bundleId")
	type Request struct {
		IsActive bool `json:"is_active"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	
	result := database.DB.Model(&models.Bundle{}).Where("id = ?", bundleID).Update("is_active", req.IsActive)
	if result.Error != nil { return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update bundle status"}) }
	if result.RowsAffected == 0 { return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Bundle not found"}) }

	return c.JSON(fiber.Map{"message": "Bundle status updated successfully."})
}