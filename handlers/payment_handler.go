package handlers

import (
	"errors"
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
				notifications.SendEmail(booking.Student.FullName, booking.Student.Email, "Your Booking is Confirmed!", "<h1>Booking Confirmed</h1><p>Your payment was successful and your class is confirmed. You will receive the meeting link shortly.</p>")
				notifications.SendEmail(booking.Teacher.FullName, booking.Teacher.Email, "You Have a New Booking!", "<h1>New Booking</h1><p>A student has booked a session with you. Please prepare for the class.</p>")
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
			go notifications.SendEmail(studentBundle.Student.FullName, studentBundle.Student.Email, "Bundle Purchase Confirmed!", "<h1>Success!</h1><p>Your class bundle purchase is complete. You can now use your class credits to book sessions.</p>")
		}

		return nil
	})

	if err != nil {
		log.Printf("🔥 CRITICAL: Error processing successful webhook for PaymentRefID %s: %v", paymentRefID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process webhook"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Webhook processed successfully"})
}


func CreatePayPalOrderHandler(c *fiber.Ctx) error {
	paymentID := c.Params("paymentId")
	if _, err := uuid.Parse(paymentID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payment ID format"})
	}

	var payment models.Payment
	if err := database.DB.Where("id = ? AND status = ? AND provider = ?", paymentID, "pending", "paypal").First(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Pending PayPal payment not found for this ID"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	order, err := payments.CreatePayPalOrder(payment.Amount, "USD") 
	if err != nil {
		log.Printf("🔥 PayPal CreateOrder API call failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create PayPal order"})
	}
	
	payment.ProviderOrderID = &order.ID
	if err := database.DB.Save(&payment).Error; err != nil {
		log.Printf("🔥 Failed to save ProviderOrderID: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update payment record"})
	}

	return c.JSON(fiber.Map{"orderID": order.ID})
}

func CapturePayPalOrderHandler(c *fiber.Ctx) error {
	type CaptureRequest struct {
		OrderID string `json:"orderID" validate:"required"`
	}
	var req CaptureRequest
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

	var payment models.Payment
	if err := database.DB.Where("provider_order_id = ?", req.OrderID).First(&payment).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Payment record not found for this order"})
	}

	capturedOrder, err := payments.CapturePayPalOrder(req.OrderID)
	if err != nil { return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()}) }
	
	if capturedOrder.Status != "COMPLETED" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Order not completed on PayPal's end"})
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		payment.Status = "succeeded"
		payment.ProviderTxnID = &capturedOrder.ID
		if err := tx.Save(&payment).Error; err != nil { return err }

		if payment.BookingID != nil {
			var booking models.Booking
			if err := tx.Preload("Student").Preload("Teacher").First(&booking, "id = ?", payment.BookingID).Error; err != nil { return err }
			booking.Status = "confirmed"
			if err := tx.Save(&booking).Error; err != nil { return err }

			go func() {
				notifications.SendEmail(booking.Student.FullName, booking.Student.Email, "Your Booking is Confirmed!", "<h1>Booking Confirmed</h1><p>Your PayPal payment was successful and your class is confirmed. You will receive the meeting link shortly.</p>")
				notifications.SendEmail(booking.Teacher.FullName, booking.Teacher.Email, "You Have a New Booking!", "<h1>New Booking</h1><p>A student has booked and paid for a session with you via PayPal.</p>")
			}()
			studentID := booking.StudentID
			go services.CompleteReferralIfApplicable(studentID)
		}
		if payment.StudentBundleID != nil {
			var studentBundle models.StudentBundle
			if err := tx.Preload("Student").First(&studentBundle, "id = ?", payment.StudentBundleID).Error; err != nil { return err }
			studentBundle.Status = "active"
			if err := tx.Save(&studentBundle).Error; err != nil { return err }
			
			go notifications.SendEmail(studentBundle.Student.FullName, studentBundle.Student.Email, "Bundle Purchase Confirmed!", "<h1>Success!</h1><p>Your class bundle purchase is complete.</p>")
			studentID := studentBundle.StudentID
			go services.CompleteReferralIfApplicable(studentID)
		}
		return nil
	})

	if err != nil { return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to finalize purchase"}) }

	return c.JSON(fiber.Map{"status": "success", "message": "Payment captured and purchase confirmed"})
}