package jobs

import (
	"log"
	"strings"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/payments"
	"github.com/anjiri1684/language_tutor/services"
)

// paystackStaleAfter is how long a Paystack checkout may sit unpaid before
// its seat is given back.
const paystackStaleAfter = 60 * time.Minute

// ReconcileStalePaystackPayments is the safety net for Paystack payments that
// stayed pending: the customer never returned to the callback page, the
// webhook was missed, or releasing the seat after a failed initialize failed.
// Each one is checked with Paystack first, so a paid order is confirmed, not
// cancelled. Only clearly unpaid ones (abandoned/failed/never created) are
// released.
func ReconcileStalePaystackPayments() {
	var stale []models.Payment
	cutoff := time.Now().Add(-paystackStaleAfter)
	if err := database.DB.
		Where("provider = ? AND status = ? AND created_at < ?", "paystack", "pending", cutoff).
		Limit(100).
		Find(&stale).Error; err != nil {
		log.Printf("🔥 Paystack reconcile: query failed: %v", err)
		return
	}

	for i := range stale {
		p := &stale[i]
		v, err := payments.VerifyPaystackTransaction(p.ID.String())
		if err != nil {
			// A reference Paystack has never seen means checkout never started.
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
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
			// "ongoing", "pending", "processing", "queued": still in flight.
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
