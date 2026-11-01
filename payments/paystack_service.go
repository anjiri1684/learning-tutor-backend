package payments

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	config "github.com/anjiri1684/language_tutor/configs"
)

const paystackBaseURL = "https://api.paystack.co"

var paystackClient = &http.Client{Timeout: 20 * time.Second}

type PaystackVerifyResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Status          string `json:"status"`
		Reference       string `json:"reference"`
		Amount          int64  `json:"amount"`
		RequestedAmount int64  `json:"requested_amount"`
		Currency        string `json:"currency"`
		Channel         string `json:"channel"`
		TransactionID   int64  `json:"id"`
		Customer        struct {
			Email string `json:"email"`
		} `json:"customer"`
	} `json:"data"`
}

type paystackInitializeResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AuthorizationURL string `json:"authorization_url"`
		AccessCode       string `json:"access_code"`
		Reference        string `json:"reference"`
	} `json:"data"`
}

// ToSubunit converts a major-unit amount (e.g. KES 1500.00) into the
// subunit integer Paystack expects (150000). Both the initialize call and
// the amount check in verify use this so the two can never disagree.
func ToSubunit(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

// PaystackCallbackURL is where Paystack sends the customer after checkout.
// The login token lives in localStorage, which is per-origin, so we send the
// user back to the exact site they paid from (apex vs www vs localhost) when
// that origin is in ALLOWED_ORIGINS. Otherwise FRONTEND_URL, then the apex.
func PaystackCallbackURL(requestOrigin string) string {
	base := ""
	origin := strings.TrimRight(strings.TrimSpace(requestOrigin), "/")
	if origin != "" {
		for _, allowed := range strings.Split(config.Config("ALLOWED_ORIGINS"), ",") {
			if strings.EqualFold(strings.TrimRight(strings.TrimSpace(allowed), "/"), origin) {
				base = origin
				break
			}
		}
	}
	if base == "" {
		base = strings.TrimRight(config.Config("FRONTEND_URL"), "/")
	}
	if base == "" {
		base = "https://phylanguagecenter.com"
	}
	return base + "/dashboard/payment/callback"
}

// InitializePaystackTransaction creates the transaction on Paystack's side
// with a server-controlled amount and returns the hosted checkout URL.
func InitializePaystackTransaction(email string, amount float64, currency, reference, callbackURL string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"email":        email,
		"amount":       ToSubunit(amount),
		"currency":     currency,
		"reference":    reference,
		"callback_url": callbackURL,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", paystackBaseURL+"/transaction/initialize", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+config.Config("PAYSTACK_SECRET_KEY"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := paystackClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result paystackInitializeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("paystack initialize: bad response (HTTP %d)", resp.StatusCode)
	}
	if !result.Status || result.Data.AuthorizationURL == "" {
		return "", fmt.Errorf("paystack initialize: %s", result.Message)
	}
	return result.Data.AuthorizationURL, nil
}

func VerifyPaystackTransaction(reference string) (*PaystackVerifyResponse, error) {
	req, err := http.NewRequest("GET", paystackBaseURL+"/transaction/verify/"+reference, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.Config("PAYSTACK_SECRET_KEY"))

	resp, err := paystackClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result PaystackVerifyResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("paystack verify: bad response (HTTP %d)", resp.StatusCode)
	}

	if !result.Status {
		return nil, fmt.Errorf("paystack: %s", result.Message)
	}

	return &result, nil
}

// ValidPaystackSignature checks the x-paystack-signature header: an
// HMAC-SHA512 of the raw request body keyed with the secret key.
func ValidPaystackSignature(body []byte, signature string) bool {
	secret := config.Config("PAYSTACK_SECRET_KEY")
	if secret == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(signature)))
}
