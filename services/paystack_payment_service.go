package services

import (
	"errors"
	"fmt"
	"html"
	"log"
	"strings"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/anjiri1684/language_tutor/payments"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Payment statuses used by the Paystack flow, beyond pending/succeeded/failed.
// Both need a human: refund, rebook, or collect the difference.
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

// alertAdmin emails ADMIN_EMAIL about a payment that needs manual action.
func alertAdmin(subject, body string) {
	to := config.Config("ADMIN_EMAIL")
	if to == "" {
		return
	}
	go notifications.SendEmail("Admin", to, subject, "<p>"+html.EscapeString(body)+"</p>")
}

// FinalizePaystackPayment confirms a payment that Paystack reports as
// successful. It is called from the callback verify, the webhook and the
// reconcile job, possibly at the same moment. The payment row is locked and
// every decision is made inside one transaction, so it serializes with
// releasePurchaseTx: a payment is either confirmed or released, never both,
// and a payment that lands after a release is always flagged.
// Returns alreadyDone=true when another call had already confirmed it.
func FinalizePaystackPayment(paymentID uuid.UUID, v *payments.PaystackVerifyResponse) (alreadyDone bool, err error) {
	if v.Data.Status != "success" {
		return false, ErrPaystackNotSuccessful
	}

	txnID := fmt.Sprintf("%d", v.Data.TransactionID)
	var outcome error
	// Side effects (emails, referrals, alerts) run only after the transaction
	// commits, so nobody gets a "confirmed" email for a purchase that rolled back.
	var afterCommit []func()

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var payment models.Payment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&payment, "id = ? AND provider = ?", paymentID, "paystack").Error; err != nil {
			return err
		}

		switch payment.Status {
		case "succeeded":
			alreadyDone = true
			return nil

		case "failed":
			// We released this order (checkout failed to start, or the
			// reconcile job timed it out) but the customer paid anyway.
			if err := tx.Model(&payment).Updates(map[string]any{
				"status": PaymentStatusPaidUnfulfilled, "provider_txn_id": txnID,
			}).Error; err != nil {
				return err
			}
			msg := fmt.Sprintf("Paystack payment %s was paid after its order was released (txn %s, %d %s). Refund or rebook the student manually.",
				payment.ID, txnID, v.Data.Amount, v.Data.Currency)
			log.Printf("🚨 CRITICAL: %s", msg)
			afterCommit = append(afterCommit, func() { alertAdmin("Paystack payment needs a refund or rebooking", msg) })
			outcome = ErrPaystackPaidAfterRelease
			return nil

		case "pending":
			// handled below

		default:
			outcome = ErrPaymentNotConfirmable
			return nil
		}

		if !PaystackAmountMatches(&payment, v) {
			// Hold the money for an admin to sort out, but give the seat back
			// so the slot isn't blocked indefinitely.
			if err := releasePurchaseTx(tx, &payment, PaymentStatusAmountMismatch); err != nil {
				return err
			}
			msg := fmt.Sprintf("Paystack payment %s amount mismatch: expected %d %s, got amount=%d requested=%d %s (txn %s). The order was cancelled; refund or collect the difference.",
				payment.ID, payments.ToSubunit(payment.Amount), payment.Currency, v.Data.Amount, v.Data.RequestedAmount, v.Data.Currency, txnID)
			log.Printf("🚨 %s", msg)
			afterCommit = append(afterCommit, func() { alertAdmin("Paystack payment amount mismatch", msg) })
			outcome = ErrPaystackAmountMismatch
			return nil
		}

		if err := tx.Model(&payment).Updates(map[string]any{
			"status": "succeeded", "provider_txn_id": txnID,
		}).Error; err != nil {
			return err
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
		log.Printf("🔥 CRITICAL: Paystack payment %s verified but finalizing failed: %v", paymentID, err)
		return false, err
	}

	for _, fn := range afterCommit {
		go fn()
	}
	return alreadyDone, outcome
}

// releasePurchaseTx moves a pending payment to newStatus and undoes what it
// was for: the booking is cancelled and its slot seat freed, or the student
// bundle is cancelled. It locks the payment row and does nothing unless the
// payment is still pending, so it never touches a paid order and serializes
// with FinalizePaystackPayment.
func releasePurchaseTx(tx *gorm.DB, p *models.Payment, newStatus string) error {
	var current models.Payment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", p.ID).Error; err != nil {
		return err
	}
	if current.Status != "pending" {
		return nil
	}
	if err := tx.Model(&current).Update("status", newStatus).Error; err != nil {
		return err
	}

	if current.StudentBundleID != nil {
		return tx.Model(&models.StudentBundle{}).
			Where("id = ? AND status = ?", current.StudentBundleID, "pending_payment").
			Update("status", "cancelled").Error
	}
	if current.BookingID == nil {
		return nil
	}

	var booking models.Booking
	if err := tx.First(&booking, "id = ?", current.BookingID).Error; err != nil {
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
}

// ReleasePendingPayment releases an unpaid order: payment -> failed, and the
// booking (seat freed) or student bundle is cancelled.
func ReleasePendingPayment(p *models.Payment) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		return releasePurchaseTx(tx, p, "failed")
	})
}

// ReleasePendingBooking releases an unpaid booking by its payment.
func ReleasePendingBooking(bookingID, paymentID uuid.UUID) error {
	return ReleasePendingPayment(&models.Payment{ID: paymentID, BookingID: &bookingID})
}

// ReleasePendingBundle releases an unpaid bundle purchase by its payment.
func ReleasePendingBundle(studentBundleID, paymentID uuid.UUID) error {
	return ReleasePendingPayment(&models.Payment{ID: paymentID, StudentBundleID: &studentBundleID})
}
