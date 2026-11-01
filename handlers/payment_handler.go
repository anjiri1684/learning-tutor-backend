package handlers

import (
	"encoding/json"
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
	"github.com/golang-jwt/jwt/v4"
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
	// Only M-Pesa payments belong to this webhook; never let it touch Paystack
	// records (e.g. flip an amount_mismatch payment to succeeded).
	if err := database.DB.Where("id = ? AND provider = ?", paymentRefID, "mpesa").First(&payment).Error; err != nil {
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

// paystackOwnerID returns the student who owns the purchase a payment is for.
func paystackOwnerID(p *models.Payment) (uuid.UUID, error) {
	switch {
	case p.BookingID != nil:
		var b models.Booking
		if err := database.DB.Select("student_id").First(&b, "id = ?", p.BookingID).Error; err != nil {
			return uuid.Nil, err
		}
		return b.StudentID, nil
	case p.StudentBundleID != nil:
		var sb models.StudentBundle
		if err := database.DB.Select("student_id").First(&sb, "id = ?", p.StudentBundleID).Error; err != nil {
			return uuid.Nil, err
		}
		return sb.StudentID, nil
	}
	return uuid.Nil, errors.New("payment has no purchase attached")
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

	paymentID, err := uuid.Parse(req.Reference)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid reference format"})
	}

	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	callerID, _ := uuid.Parse(fmt.Sprint(claims["user_id"]))

	notFound := c.Status(fiber.StatusNotFound).JSON
	var payment models.Payment
	if err := database.DB.Where("id = ? AND provider = ?", paymentID, "paystack").First(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return notFound(fiber.Map{"error": "Paystack payment not found for this reference"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	// Only the student who made the purchase may confirm it. Answer with the
	// same 404 so references can't be probed.
	ownerID, err := paystackOwnerID(&payment)
	if err != nil || ownerID != callerID {
		return notFound(fiber.Map{"error": "Paystack payment not found for this reference"})
	}

	purchaseType := services.PurchaseType(&payment)

	// The callback page can be refreshed or opened twice; treat an
	// already-confirmed payment as success instead of an error.
	if payment.Status == "succeeded" {
		return c.JSON(fiber.Map{"status": "success", "type": purchaseType, "message": "Payment already confirmed"})
	}

	verified, err := payments.VerifyPaystackTransaction(req.Reference)
	if err != nil {
		log.Printf("🔥 Paystack verify API call failed: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "Failed to verify payment with Paystack"})
	}
	if verified.Data.Status != "success" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Payment not completed on Paystack (status: " + verified.Data.Status + ")"})
	}

	alreadyDone, err := services.FinalizePaystackPayment(payment.ID, verified)
	switch {
	case errors.Is(err, services.ErrPaystackAmountMismatch):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Paid amount does not match the order amount. Please contact support."})
	case errors.Is(err, services.ErrPaystackPaidAfterRelease):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Your payment was received after this order expired. Please contact support for a refund or rebooking."})
	case errors.Is(err, services.ErrPaymentNotConfirmable):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "This payment can no longer be confirmed. Please contact support."})
	case err != nil:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to finalize purchase"})
	}

	msg := "Payment verified and purchase confirmed"
	if alreadyDone {
		msg = "Payment already confirmed"
	}
	return c.JSON(fiber.Map{"status": "success", "type": purchaseType, "message": msg})
}

// PaystackWebhookHandler receives Paystack events so payments are confirmed
// even when the customer never comes back to the callback page.
// Configure in the Paystack dashboard: <WEBHOOK_BASE_URL>/api/v1/payments/webhooks/paystack
func PaystackWebhookHandler(c *fiber.Ctx) error {
	body := c.Body()
	if !payments.ValidPaystackSignature(body, c.Get("x-paystack-signature")) {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	var event struct {
		Event string `json:"event"`
		Data  struct {
			Reference string `json:"reference"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	if event.Event != "charge.success" {
		return c.SendStatus(fiber.StatusOK)
	}

	paymentID, err := uuid.Parse(event.Data.Reference)
	if err != nil {
		// Not one of ours (e.g. a payment made outside this app).
		return c.SendStatus(fiber.StatusOK)
	}

	var payment models.Payment
	if err := database.DB.Where("id = ? AND provider = ?", paymentID, "paystack").First(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if payment.Status == "succeeded" {
		return c.SendStatus(fiber.StatusOK)
	}

	// Re-fetch from Paystack rather than trusting the event body's figures.
	verified, err := payments.VerifyPaystackTransaction(event.Data.Reference)
	if err != nil {
		log.Printf("🔥 Paystack webhook: verify %s failed: %v", paymentID, err)
		return c.SendStatus(fiber.StatusBadGateway) // non-2xx → Paystack retries
	}

	if _, err := services.FinalizePaystackPayment(paymentID, verified); err != nil {
		switch {
		case errors.Is(err, services.ErrPaystackAmountMismatch),
			errors.Is(err, services.ErrPaystackPaidAfterRelease),
			errors.Is(err, services.ErrPaymentNotConfirmable),
			errors.Is(err, services.ErrPaystackNotSuccessful):
			return c.SendStatus(fiber.StatusOK) // final state; retrying won't help
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.SendStatus(fiber.StatusOK)
}
