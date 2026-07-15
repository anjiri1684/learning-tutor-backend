package handlers

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/anjiri1684/language_tutor/payments"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type KcbWebhookPayload struct {
	Body struct {
		StkCallback struct {
			MerchantRequestID string `json:"MerchantRequestID"`
			CheckoutRequestID string `json:"CheckoutRequestID"`
			ResultCode        int    `json:"ResultCode"`
			ResultDesc        string `json:"ResultDesc"`
			CallbackMetadata  struct {
				Item []struct {
					Name  string      `json:"Name"`
					Value interface{} `json:"Value"`
				} `json:"Item"` 
			} `json:"CallbackMetadata"`
			Reference string `json:"Reference"`
		} `json:"stkCallback"`
	} `json:"Body"`
}

func HandlePaymentWebhook(c *fiber.Ctx) error {
	var payload KcbWebhookPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse webhook payload"})
	}

	stk := payload.Body.StkCallback

	var paymentRefID string
	parts := strings.Split(stk.Reference, "-")
	if len(parts) == 2 {
		paymentRefID = parts[1] 
	} else {
		paymentRefID = stk.Reference 
	}

	log.Printf("Received webhook for MerchantRequestID: %s, PaymentRefID: %s, ResultCode: %d",
		stk.MerchantRequestID, paymentRefID, stk.ResultCode)

	var payment models.Payment
	if err := database.DB.Where("id = ?", paymentRefID).First(&payment).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Payment record not found"})
	}

	if payment.Status == "succeeded" {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Webhook already processed"})
	}

	if stk.ResultCode != 0 {
		payment.Status = "failed"
		database.DB.Save(&payment)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Acknowledged failed payment"})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var mpesaReceipt string
		for _, item := range stk.CallbackMetadata.Item {
			if item.Name == "MpesaReceiptNumber" {
				if val, ok := item.Value.(string); ok {
					mpesaReceipt = val
					break
				}
			}
		}

		payment.Status = "succeeded"
		payment.ProviderTxnID = &mpesaReceipt
		payment.MerchantRequestID = &stk.MerchantRequestID 
		if err := tx.Save(&payment).Error; err != nil {
			return err
		}

		if payment.BookingID != nil {
			var booking models.Booking
			if err := tx.Preload("Student").Preload("Teacher").First(&booking, "id = ?", payment.BookingID).Error; err != nil {
				return err
			}
			booking.Status = "confirmed"
			if err := tx.Save(&booking).Error; err != nil {
				return err
			}
			go func() {
				sSub, sHtml := notifications.BookingConfirmedStudentTemplate(booking.Student.FullName)
				notifications.SendEmail(booking.Student.FullName, booking.Student.Email, sSub, sHtml)
				tSub, tHtml := notifications.BookingConfirmedTeacherTemplate(booking.Teacher.FullName)
				notifications.SendEmail(booking.Teacher.FullName, booking.Teacher.Email, tSub, tHtml)
			}()
		}

		if payment.StudentBundleID != nil {
			var studentBundle models.StudentBundle
			if err := tx.Preload("Student").First(&studentBundle, "id = ?", payment.StudentBundleID).Error; err != nil {
				return err
			}
			studentBundle.Status = "active"
			if err := tx.Save(&studentBundle).Error; err != nil {
				return err
			}
			go func() {
				subject, html := notifications.BundlePurchasedTemplate(studentBundle.Student.FullName)
				notifications.SendEmail(studentBundle.Student.FullName, studentBundle.Student.Email, subject, html)
			}()
		}

		return nil
	})

	if err != nil {
		log.Printf("🔥 CRITICAL: Error processing successful webhook for PaymentRefID %s: %v", paymentRefID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process webhook"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Webhook processed successfully"})
}


func VerifyPaystackPaymentHandler(c *fiber.Ctx) error {
	type VerifyRequest struct {
		Reference string `json:"reference" validate:"required"`
	}
	var req VerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if _, err := uuid.Parse(req.Reference); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid reference format"})
	}

	var payment models.Payment
	if err := database.DB.Where("id = ? AND provider = ? AND status = ?", req.Reference, "paystack", "pending").First(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Pending Paystack payment not found for this reference"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	verified, err := payments.VerifyPaystackTransaction(req.Reference)
	if err != nil {
		log.Printf("🔥 Paystack verify API call failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to verify payment with Paystack"})
	}

	if verified.Data.Status != "success" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Payment not completed on Paystack"})
	}

	txnID := fmt.Sprintf("%d", verified.Data.TransactionID)

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		payment.Status = "succeeded"
		payment.ProviderTxnID = &txnID
		if err := tx.Save(&payment).Error; err != nil {
			return err
		}

		if payment.BookingID != nil {
			var booking models.Booking
			if err := tx.Preload("Student").Preload("Teacher").First(&booking, "id = ?", payment.BookingID).Error; err != nil {
				return err
			}
			booking.Status = "confirmed"
			if err := tx.Save(&booking).Error; err != nil {
				return err
			}
			go func() {
				sSub, sHtml := notifications.BookingConfirmedStudentTemplate(booking.Student.FullName)
				notifications.SendEmail(booking.Student.FullName, booking.Student.Email, sSub, sHtml)
				tSub, tHtml := notifications.BookingConfirmedTeacherTemplate(booking.Teacher.FullName)
				notifications.SendEmail(booking.Teacher.FullName, booking.Teacher.Email, tSub, tHtml)
			}()
			studentID := booking.StudentID
			go services.CompleteReferralIfApplicable(studentID)
		}
		if payment.StudentBundleID != nil {
			var studentBundle models.StudentBundle
			if err := tx.Preload("Student").First(&studentBundle, "id = ?", payment.StudentBundleID).Error; err != nil {
				return err
			}
			studentBundle.Status = "active"
			if err := tx.Save(&studentBundle).Error; err != nil {
				return err
			}
			go func() {
				subject, html := notifications.BundlePurchasedTemplate(studentBundle.Student.FullName)
				notifications.SendEmail(studentBundle.Student.FullName, studentBundle.Student.Email, subject, html)
			}()
			studentID := studentBundle.StudentID
			go services.CompleteReferralIfApplicable(studentID)
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to finalize purchase"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Payment verified and purchase confirmed"})
}