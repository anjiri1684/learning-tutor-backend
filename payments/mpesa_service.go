package payments

import (
	"bytes"
	"encoding/json"
	"errors" 
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings" 
	"time"

	config "github.com/anjiri1684/language_tutor/configs"
)


// const kcbUatBaseURL = "https://uat.buni.kcbgroup.com/mm/api/request/1.0.0"
const kcbUatBaseURL = "https://api.buni.kcbgroup.com/mm/api/request/1.0.0"

type StkPushRequest struct {
	PhoneNumber            string `json:"phoneNumber"`
	Amount                 string `json:"amount"`
	InvoiceNumber          string `json:"invoiceNumber"`
	SharedShortCode        bool   `json:"sharedShortCode"`
	OrgShortCode           string `json:"orgShortCode"`
	OrgPassKey             string `json:"orgPassKey"`
	CallbackURL            string `json:"callbackUrl"`
	TransactionDescription string `json:"transactionDescription"`
}

type StkPushResponse struct {
	Header struct {
		StatusCode        string `json:"statusCode"`
		StatusDescription string `json:"statusDescription"`
	} `json:"header"`
	Response struct {
		MerchantRequestID   string `json:"MerchantRequestID"`
		CheckoutRequestID   string `json:"CheckoutRequestID"`
		CustomerMessage     string `json:"CustomerMessage"`
		ResponseCode        string `json:"ResponseCode"`
		ResponseDescription string `json:"ResponseDescription"`
	} `json:"response"`
}

var nonNumericRegex = regexp.MustCompile(`[^0-9]`)

func SanitizeMpesaNumber(phone string) (string, error) {
	sanitized := nonNumericRegex.ReplaceAllString(phone, "")

	if (strings.HasPrefix(sanitized, "07") || strings.HasPrefix(sanitized, "01")) && len(sanitized) == 10 {
		return "254" + sanitized[1:], nil
	}
	if (strings.HasPrefix(sanitized, "7") || strings.HasPrefix(sanitized, "1")) && len(sanitized) == 9 {
		return "254" + sanitized, nil
	}
	if strings.HasPrefix(sanitized, "254") && len(sanitized) == 12 {
		return sanitized, nil
	}

	return "", errors.New("invalid M-Pesa phone number format")
}

func InitiateMpesaSTKPush(amount float64, phoneNumber string, paymentRefID string) (*StkPushResponse, error) {
	accessToken, err := GetKcbAccessToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get KCB access token: %v", err)
	}

	sanitizedPhone, err := SanitizeMpesaNumber(phoneNumber)
	if err != nil {
		return nil, err
	}

	callbackURL := config.Config("WEBHOOK_BASE_URL") + "/api/v1/payments/webhook"
	amountStr := strconv.FormatFloat(amount, 'f', 0, 64)

	kcbAccount := config.Config("KCB_ACCOUNT_NUMBER")
	if kcbAccount == "" {
		return nil, fmt.Errorf("KCB_ACCOUNT_NUMBER is not set in .env")
	}
	invoiceNumber := fmt.Sprintf("%s-%s", kcbAccount, paymentRefID)

	payload := StkPushRequest{
		PhoneNumber:            sanitizedPhone,
		Amount:                 amountStr,
		InvoiceNumber:          invoiceNumber,
		SharedShortCode:        true,
		CallbackURL:            callbackURL,
		TransactionDescription: config.Config("KCB_TRANSACTION_DESC"),
	}

	body, err := json.Marshal(payload)
	if err != nil { return nil, fmt.Errorf("failed to marshal STK payload: %v", err) }

	log.Println("Request Body:", bytes.NewBuffer(body))

	req, err := http.NewRequest("POST", kcbUatBaseURL+"/stkpush", bytes.NewBuffer(body))
	if err != nil { return nil, fmt.Errorf("failed to create STK request: %v", err) }

	messageID := fmt.Sprintf("%s_%d", paymentRefID, time.Now().UnixNano())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("routeCode", config.Config("KCB_ROUTE_CODE"))
	req.Header.Set("operation", "STKPush")
	req.Header.Set("messageId", messageID)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{ Timeout: 10 * time.Second }
	resp, err := client.Do(req)
	if err != nil { return nil, fmt.Errorf("failed to send STK request: %v", err) }
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil { return nil, fmt.Errorf("failed to read STK response body: %v", err) }

	fmt.Println("Response Body:", string(respBody))


	if resp.StatusCode != http.StatusOK {
		log.Printf("KCB API Error: %s", string(respBody))
		return nil, fmt.Errorf("KCB Buni API returned non-200 status: %d", resp.StatusCode)
	}

	var stkResponse StkPushResponse
	if err := json.Unmarshal(respBody, &stkResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal STK response: %v", err)
	}
	
	if stkResponse.Response.ResponseCode != "0" { 
		log.Printf("KCB STK Push initiation failed: %s", stkResponse.Response.ResponseDescription)
		return nil, fmt.Errorf("KCB STK Push failed: %s", stkResponse.Response.ResponseDescription)
	}

	log.Println("✅ STK Push initiated successfully for payment:", paymentRefID)
	return &stkResponse, nil
}