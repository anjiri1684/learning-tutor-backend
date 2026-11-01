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
			services.ApplyDefaultMeetingLink(tx, &booking)
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
	if err := database.DB.Where("id = ? AND provider = ?", req.Reference, "paystack").First(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Paystack payment not found for this reference"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	purchaseType := "booking"
	if payment.StudentBundleID != nil {
		purchaseType = "bundle"
	}

	// The callback page can be refreshed or opened twice; treat an
	// already-confirmed payment as success instead of a 404.
	if payment.Status == "succeeded" {
		return c.JSON(fiber.Map{"status": "success", "type": purchaseType, "message": "Payment already confirmed"})
	}
	if payment.Status != "pending" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "This payment can no longer be confirmed (status: " + payment.Status + "). Please contact support."})
	}

	verified, err := payments.VerifyPaystackTransaction(req.Reference)
	if err != nil {
		log.Printf("🔥 Paystack verify API call failed: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "Failed to verify payment with Paystack"})
	}

	if verified.Data.Status != "success" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Payment not completed on Paystack (status: " + verified.Data.Status + ")"})
	}

	// Amount check: what Paystack actually collected must match what we
	// charged for, in the same currency. Paystack amounts are in subunits.
	expected := payments.ToSubunit(payment.Amount)
	if verified.Data.Amount != expected || !strings.EqualFold(verified.Data.Currency, payment.Currency) {
		log.Printf("🚨 Paystack amount mismatch for payment %s: expected %d %s, got %d %s (txn %d)",
			payment.ID, expected, payment.Currency, verified.Data.Amount, verified.Data.Currency, verified.Data.TransactionID)
		database.DB.Model(&models.Payment{}).Where("id = ? AND status = ?", payment.ID, "pending").Update("status", "amount_mismatch")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Paid amount does not match the order amount. Please contact support."})
	}

	txnID := fmt.Sprintf("%d", verified.Data.TransactionID)
	alreadyDone := false

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// Conditional update so two concurrent verifies can't both confirm
		// (and both send emails). Only the one that flips pending wins.
		res := tx.Model(&models.Payment{}).
			Where("id = ? AND status = ?", payment.ID, "pending").
			Updates(map[string]interface{}{"status": "succeeded", "provider_txn_id": txnID})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			alreadyDone = true
			return nil
		}

		if payment.BookingID != nil {
			var booking models.Booking
			if err := tx.Preload("Student").Preload("Teacher").First(&booking, "id = ?", payment.BookingID).Error; err != nil {
				return err
			}
			booking.Status = "confirmed"
			services.ApplyDefaultMeetingLink(tx, &booking)
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
		log.Printf("🔥 CRITICAL: Paystack payment %s verified but finalizing failed: %v", payment.ID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to finalize purchase"})
	}

	msg := "Payment verified and purchase confirmed"
	if alreadyDone {
		msg = "Payment already confirmed"
	}
	return c.JSON(fiber.Map{"status": "success", "type": purchaseType, "message": msg})
}
