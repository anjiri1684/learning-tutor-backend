package jobs

import (
	"errors"
	"log"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/payments"
	"github.com/anjiri1684/language_tutor/services"
)

const (
	// paystackStaleAfter is how long a Paystack checkout may sit unpaid before
	// its seat is given back.
	paystackStaleAfter = 60 * time.Minute
	// paystackMaxPending: after this, even a transaction Paystack still calls
	// "ongoing"/"processing" is released. If it is somehow paid later,
	// FinalizePaystackPayment flags it paid_unfulfilled and alerts the admin.
	paystackMaxPending = 24 * time.Hour
	paystackBatchSize  = 100
)

// ReconcileStalePaystackPayments is the safety net for Paystack payments that
// stayed pending: the customer never returned to the callback page, the
// webhook was missed, or releasing the seat after a failed initialize failed.
// Each one is checked with Paystack first, so a paid order is confirmed, not
// cancelled. Only clearly unpaid ones are released.
func ReconcileStalePaystackPayments() {
	var stale []models.Payment
	now := time.Now()
	if err := database.DB.
		Where("provider = ? AND status = ? AND created_at < ?", "paystack", "pending", now.Add(-paystackStaleAfter)).
		// Least recently checked first; each row is touched below, so rows Paystack
		// keeps erroring on rotate to the back instead of blocking newer ones.
		Order("updated_at ASC").
		Limit(paystackBatchSize).
		Find(&stale).Error; err != nil {
		log.Printf("🔥 Paystack reconcile: query failed: %v", err)
		return
	}

	for i := range stale {
		p := &stale[i]
		database.DB.Model(&models.Payment{}).Where("id = ?", p.ID).UpdateColumn("updated_at", now)
		v, err := payments.VerifyPaystackTransaction(p.ID.String())
		if err != nil {
			// Paystack explicitly reports it has no such transaction: the
			// checkout was never created. Any other error is retried next run.
			if errors.Is(err, payments.ErrPaystackTxnNotFound) {
				releaseStale(p, "no transaction on Paystack")
				continue
			}
			log.Printf("Paystack reconcile: verify %s failed, will retry: %v", p.ID, err)
			continue
		}

		switch v.Data.Status {
		case "success":
			if _, err := services.FinalizePaystackPayment(p.ID, v); err != nil {
				log.Printf("Paystack reconcile: finalize %s: %v", p.ID, err)
			} else {
				log.Printf("✅ Paystack reconcile: confirmed %s (customer did not return to callback)", p.ID)
			}
		case "abandoned", "failed", "reversed":
			releaseStale(p, v.Data.Status)
		default:
			// "ongoing", "pending", "processing", "queued": still in flight,
			// unless it has been far too long.
			if p.CreatedAt.Before(now.Add(-paystackMaxPending)) {
				releaseStale(p, "still "+v.Data.Status+" after 24h")
			}
		}
	}
}

func releaseStale(p *models.Payment, reason string) {
	if err := services.ReleasePendingPayment(p); err != nil {
		log.Printf("🔥 Paystack reconcile: release %s failed: %v", p.ID, err)
		return
	}
	log.Printf("Paystack reconcile: released %s (%s)", p.ID, reason)
}
