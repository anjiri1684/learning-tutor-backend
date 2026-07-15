package payments

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	config "github.com/anjiri1684/language_tutor/configs"
)

const paystackBaseURL = "https://api.paystack.co"

type PaystackVerifyResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Status        string  `json:"status"`
		Reference     string  `json:"reference"`
		Amount        int     `json:"amount"`
		Currency      string  `json:"currency"`
		Channel       string  `json:"channel"`
		TransactionID int64   `json:"id"`
		Customer      struct {
			Email string `json:"email"`
		} `json:"customer"`
	} `json:"data"`
}

func VerifyPaystackTransaction(reference string) (*PaystackVerifyResponse, error) {
	secretKey := config.Config("PAYSTACK_SECRET_KEY")

	req, err := http.NewRequest("GET", paystackBaseURL+"/transaction/verify/"+reference, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+secretKey)

	resp, err := http.DefaultClient.Do(req)
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
		return nil, err
	}

	if !result.Status {
		return nil, fmt.Errorf("paystack: %s", result.Message)
	}

	return &result, nil
}
