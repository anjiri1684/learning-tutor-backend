package middleware

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newTestApp() *fiber.App {
	app := fiber.New()
	app.Post("/webhook", KCBWebhookIPAllowlist(), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func TestKCBWebhookIPAllowlist_RejectsWhenUnconfigured(t *testing.T) {
	os.Unsetenv("KCB_TRUSTED_IPS")
	app := newTestApp()

	req := httptest.NewRequest("POST", "/webhook", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403 when KCB_TRUSTED_IPS unset, got %d", resp.StatusCode)
	}
}

func TestKCBWebhookIPAllowlist_RejectsUntrustedIP(t *testing.T) {
	os.Setenv("KCB_TRUSTED_IPS", "196.201.214.200,196.201.214.206")
	defer os.Unsetenv("KCB_TRUSTED_IPS")
	app := newTestApp()

	req := httptest.NewRequest("POST", "/webhook", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403 for untrusted IP, got %d", resp.StatusCode)
	}
}

func TestKCBWebhookIPAllowlist_AllowsTrustedIP(t *testing.T) {
	os.Setenv("KCB_TRUSTED_IPS", "196.201.214.200,196.201.214.206")
	defer os.Unsetenv("KCB_TRUSTED_IPS")
	app := newTestApp()

	req := httptest.NewRequest("POST", "/webhook", nil)
	req.Header.Set("X-Forwarded-For", "196.201.214.206")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for trusted IP, got %d", resp.StatusCode)
	}
}

func TestKCBWebhookIPAllowlist_UsesFirstForwardedForHop(t *testing.T) {
	os.Setenv("KCB_TRUSTED_IPS", "196.201.214.200")
	defer os.Unsetenv("KCB_TRUSTED_IPS")
	app := newTestApp()

	// A real chain looks like "<original client>, <intermediate proxies...>".
	// Only the first entry should be checked against the allowlist.
	req := httptest.NewRequest("POST", "/webhook", nil)
	req.Header.Set("X-Forwarded-For", "196.201.214.200, 10.0.0.1")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 when trusted IP is first hop, got %d", resp.StatusCode)
	}
}
