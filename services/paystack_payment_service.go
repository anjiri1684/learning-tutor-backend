package services

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/anjiri1684/language_tutor/payments"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Payment statuses used by the Paystack flow, beyond pending/succeeded/failed.
const (
	PaymentStatusAmountMismatch  = "amount_mismatch"
	PaymentStatusPaidUnfulfilled = "paid_unfulfilled"
)

var (
	ErrPaystackNotSuccessful  = errors.New("payment not completed on Paystack")
	ErrPaystackAmountMismatch = errors.New("paid amount does not match the order amount")
	// ErrPaystackPaidAfterRelease means the customer paid for an order we had
	// already released (seat freed). The money needs a manual refund or rebooking.
	ErrPaystackPaidAfterRelease = errors.New("payment received for an order that was already released")
	ErrPaymentNotConfirmable    = errors.New("payment can no longer be confirmed")
)

// PurchaseType reports what a payment is for: "booking" or "bundle".
func PurchaseType(p *models.Payment) string {
	if p.StudentBundleID != nil {
		return "bundle"
	}
	return "booking"
}

// PaystackAmountMatches checks what Paystack collected against the order.
// When "pass fees to customer" is on, data.amount includes Paystack's fee and
// data.requested_amount holds the order amount, so either may match exactly.
func PaystackAmountMatches(p *models.Payment, v *payments.PaystackVerifyResponse) bool {
	if !strings.EqualFold(v.Data.Currency, p.Currency) {
		return false
	}
	expected := payments.ToSubunit(p.Amount)
	if v.Data.Amount == expected {
		return true
	}
	return v.Data.RequestedAmount == expected && v.Data.Amount >= expected
}

// FinalizePaystackPayment confirms a payment that Paystack reports as
// successful. It is safe to call more than once and from several places
// (callback verify, webhook, sweep job): only the call that flips the payment
// from pending to succeeded does the work and sends emails.
// Returns alreadyDone=true when another call had already confirmed it.
func FinalizePaystackPayment(paymentID uuid.UUID, v *payments.PaystackVerifyResponse) (alreadyDone bool, err error) {
	var payment models.Payment
	if err := database.DB.First(&payment, "id = ? AND provider = ?", paymentID, "paystack").Error; err != nil {
		return false, err
	}

	if v.Data.Status != "success" {
		return false, ErrPaystackNotSuccessful
	}

	switch payment.Status {
	case "succeeded":
		return true, nil
	case "pending":
		// handled below
	case "failed":
		// We released this order (checkout failed to start, or the sweep job
		// timed it out) but the customer paid anyway. Flag it for a refund.
		database.DB.Model(&models.Payment{}).
			Where("id = ? AND status = ?", payment.ID, "failed").
			Updates(map[string]any{"status": PaymentStatusPaidUnfulfilled, "provider_txn_id": fmt.Sprintf("%d", v.Data.TransactionID)})
		log.Printf("🚨 CRITICAL: Paystack payment %s was paid after the order was released (txn %d, %d %s). Refund or rebook manually.",
			payment.ID, v.Data.TransactionID, v.Data.Amount, v.Data.Currency)
		return false, ErrPaystackPaidAfterRelease
	default:
		return false, ErrPaymentNotConfirmable
	}

	if !PaystackAmountMatches(&payment, v) {
		log.Printf("🚨 Paystack amount mismatch for payment %s: expected %d %s, got amount=%d requested=%d %s (txn %d)",
			payment.ID, payments.ToSubunit(payment.Amount), payment.Currency, v.Data.Amount, v.Data.RequestedAmount, v.Data.Currency, v.Data.TransactionID)
		if err := database.DB.Model(&models.Payment{}).Where("id = ? AND status = ?", payment.ID, "pending").Update("status", PaymentStatusAmountMismatch).Error; err != nil {
			log.Printf("🔥 could not mark payment %s as amount_mismatch: %v", payment.ID, err)
		}
		return false, ErrPaystackAmountMismatch
	}

	txnID := fmt.Sprintf("%d", v.Data.TransactionID)
	// Side effects (emails, referrals) run only after the transaction commits,
	// so nobody gets a "confirmed" email for a purchase that rolled back.
	var afterCommit []func()

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Payment{}).
			Where("id = ? AND status = ?", payment.ID, "pending").
			Updates(map[string]any{"status": "succeeded", "provider_txn_id": txnID})
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
			ApplyDefaultMeetingLink(tx, &booking)
			if err := tx.Save(&booking).Error; err != nil {
				return err
			}
			afterCommit = append(afterCommit, func() {
				sSub, sHtml := notifications.BookingConfirmedStudentTemplate(booking.Student.FullName)
				notifications.SendEmail(booking.Student.FullName, booking.Student.Email, sSub, sHtml)
				tSub, tHtml := notifications.BookingConfirmedTeacherTemplate(booking.Teacher.FullName)
				notifications.SendEmail(booking.Teacher.FullName, booking.Teacher.Email, tSub, tHtml)
			}, func() { CompleteReferralIfApplicable(booking.StudentID) })
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
			afterCommit = append(afterCommit, func() {
				subject, html := notifications.BundlePurchasedTemplate(studentBundle.Student.FullName)
				notifications.SendEmail(studentBundle.Student.FullName, studentBundle.Student.Email, subject, html)
			}, func() { CompleteReferralIfApplicable(studentBundle.StudentID) })
		}
		return nil
	})
	if err != nil {
		log.Printf("🔥 CRITICAL: Paystack payment %s verified but finalizing failed: %v", payment.ID, err)
		return false, err
	}

	for _, fn := range afterCommit {
		go fn()
	}
	return alreadyDone, nil
}

// ReleasePendingBooking undoes a booking whose external payment never
// happened: payment -> failed, booking -> cancelled, and the slot seat is
// freed. Guarded on the pending statuses so it never touches a paid booking.
func ReleasePendingBooking(bookingID, paymentID uuid.UUID) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Payment{}).
			Where("id = ? AND status = ?", paymentID, "pending").
			Update("status", "failed")
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}

		var booking models.Booking
		if err := tx.First(&booking, "id = ?", bookingID).Error; err != nil {
			return err
		}
		if booking.Status != "pending_payment" {
			return nil
		}
		booking.Status = "cancelled"
		if err := tx.Save(&booking).Error; err != nil {
			return err
		}

		var slot models.AvailabilitySlot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&slot, "id = ?", booking.AvailabilitySlotID).Error; err != nil {
			return err
		}
		if slot.CurrentStudents > 0 {
			slot.CurrentStudents--
		}
		if slot.CurrentStudents < slot.MaxStudents {
			slot.Status = "available"
		}
		return tx.Save(&slot).Error
	})
}

// ReleasePendingBundle closes out a bundle purchase whose payment never
// happened: payment -> failed, student bundle -> cancelled.
func ReleasePendingBundle(studentBundleID, paymentID uuid.UUID) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Payment{}).
			Where("id = ? AND status = ?", paymentID, "pending").
			Update("status", "failed")
		if res.Error != nil || res.RowsAffected == 0 {
			return res.Error
		}
		return tx.Model(&models.StudentBundle{}).
			Where("id = ? AND status = ?", studentBundleID, "pending_payment").
			Update("status", "cancelled").Error
	})
}

// ReleasePendingPayment releases whatever the payment was for.
func ReleasePendingPayment(p *models.Payment) error {
	switch {
	case p.BookingID != nil:
		return ReleasePendingBooking(*p.BookingID, p.ID)
	case p.StudentBundleID != nil:
		return ReleasePendingBundle(*p.StudentBundleID, p.ID)
	}
	return nil
}
