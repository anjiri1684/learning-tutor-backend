package services

import (
	"testing"

	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/payments"
)

func verifyResp(amount, requested int64, currency string) *payments.PaystackVerifyResponse {
	v := &payments.PaystackVerifyResponse{}
	v.Data.Amount = amount
	v.Data.RequestedAmount = requested
	v.Data.Currency = currency
	return v
}

func TestPaystackAmountMatches(t *testing.T) {
	p := &models.Payment{Amount: 1500, Currency: "KES"}
	cases := []struct {
		name string
		v    *payments.PaystackVerifyResponse
		want bool
	}{
		{"exact", verifyResp(150000, 150000, "KES"), true},
		{"exact, no requested_amount", verifyResp(150000, 0, "KES"), true},
		{"fees passed to customer", verifyResp(152175, 150000, "KES"), true},
		{"underpaid", verifyResp(100, 100, "KES"), false},
		{"underpaid but requested matches", verifyResp(100, 150000, "KES"), false},
		{"wrong currency", verifyResp(150000, 150000, "USD"), false},
		{"currency case-insensitive", verifyResp(150000, 150000, "kes"), true},
	}
	for _, c := range cases {
		if got := PaystackAmountMatches(p, c.v); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
