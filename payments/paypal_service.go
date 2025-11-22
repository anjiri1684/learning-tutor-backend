package payments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	config "github.com/anjiri1684/language_tutor/configs"
)

type PayPalOrder struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
}

func getPayPalAccessToken() (string, error) {
	apiBase := config.Config("PAYPAL_API_BASE_URL")
	clientID := config.Config("PAYPAL_CLIENT_ID")
	clientSecret := config.Config("PAYPAL_CLIENT_SECRET")

	reqBody := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/oauth2/token", apiBase), reqBody)
	if err != nil { return "", err }

	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get access token, status: %s", resp.Status)
	}

	var tokenResp accessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}

func CreatePayPalOrder(amount float64, currency string) (*PayPalOrder, error) {
	accessToken, err := getPayPalAccessToken()
	if err != nil { return nil, err }

	apiBase := config.Config("PAYPAL_API_BASE_URL")
	
	amountStr := fmt.Sprintf("%.2f", amount)

	payload := map[string]interface{}{
		"intent": "CAPTURE",
		"purchase_units": []map[string]interface{}{
			{
				"amount": map[string]string{
					"currency_code": currency, 
					"value":         amountStr,
				},
			},
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v2/checkout/orders", apiBase), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create order: %s", string(respBody))
	}

	var order PayPalOrder
	json.NewDecoder(resp.Body).Decode(&order)
	return &order, nil
}

func CapturePayPalOrder(orderID string) (*PayPalOrder, error) {
	accessToken, err := getPayPalAccessToken()
	if err != nil { return nil, err }

	apiBase := config.Config("PAYPAL_API_BASE_URL")

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v2/checkout/orders/%s/capture", apiBase, orderID), nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to capture order: %s", string(respBody))
	}

	var order PayPalOrder
	json.NewDecoder(resp.Body).Decode(&order)
	return &order, nil
}