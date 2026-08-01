package middleware

import (
	"log"
	"strings"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/gofiber/fiber/v2"
)

// KCBWebhookIPAllowlist restricts a route to requests originating from KCB's
// documented M-Pesa callback IP ranges (KCB_TRUSTED_IPS, comma-separated).
// Without this, anyone who discovers the webhook URL and a valid payment
// reference could forge a "payment succeeded" callback and get a free booking.
func KCBWebhookIPAllowlist() fiber.Handler {
	return func(c *fiber.Ctx) error {
		trustedIPsEnv := config.Config("KCB_TRUSTED_IPS")
		if trustedIPsEnv == "" {
			log.Printf("[SECURITY] KCB_TRUSTED_IPS is not set; rejecting webhook request from %s", c.IP())
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Webhook not configured"})
		}

		trusted := make(map[string]bool)
		for _, ip := range strings.Split(trustedIPsEnv, ",") {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				trusted[ip] = true
			}
		}

		requestIP := ClientIP(c)
		if !trusted[requestIP] {
			log.Printf("[SECURITY] Rejected webhook request from untrusted IP: %s", requestIP)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
		}

		return c.Next()
	}
}

// ClientIP resolves the real caller's IP. This service is deployed on Render
// (see WEBHOOK_BASE_URL), where the app process has no direct public port —
// every request necessarily passes through Render's edge proxy first, which
// sets X-Forwarded-For to the true client IP. That makes trusting the header
// here safe in this specific hosting setup; it would NOT be safe on a host
// where the app's port is directly internet-reachable, since then any caller
// could set that header themselves to bypass this allowlist. If this service
// is ever moved off Render (or put behind a different proxy), revisit this.
func ClientIP(c *fiber.Ctx) string {
	forwardedFor := c.Get("X-Forwarded-For")
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}
	return c.IP()
}
