package notifications

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "strings"
    "time"

    config "github.com/anjiri1684/language_tutor/configs"
)

type BrevoService struct {
    APIKey      string
    SenderEmail string
    SenderName  string
}

var EmailClient *BrevoService

type brevoPayload struct {
    Sender      map[string]string   `json:"sender"`
    To          []map[string]string `json:"to"`
    Subject     string              `json:"subject"`
    HTMLContent string              `json:"htmlContent"`
}

func InitEmailService() {
    apiKey := config.Config("BREVO_API_KEY")
    senderEmail := config.Config("EMAIL_SENDER")
    senderName := config.Config("EMAIL_SENDER_NAME")

    log.Printf("Initializing email service with API Key: %s..., Sender Email: %s, Sender Name: %s", apiKey[:8], senderEmail, senderName)

    if apiKey == "" || senderEmail == "" || senderName == "" {
        log.Println("⚠️ Email service not configured. Missing API Key, Sender Email, or Sender Name.")
        EmailClient = nil
        return
    }

    EmailClient = &BrevoService{
        APIKey:      apiKey,
        SenderEmail: senderEmail,
        SenderName:  senderName,
    }
    log.Println("✅ Email service initialized successfully.")
}

func (s *BrevoService) send(toEmail, toName, subject, htmlContent string) error {
    url := "https://api.brevo.com/v3/smtp/email"

    if toEmail == "" || !strings.Contains(toEmail, "@") {
        return fmt.Errorf("invalid recipient email: %s", toEmail)
    }

    recipientName := toName
    if recipientName == "" {
        recipientName = toEmail[:strings.Index(toEmail, "@")]
    }

    payload := brevoPayload{
        Sender:      map[string]string{"name": s.SenderName, "email": s.SenderEmail},
        To:          []map[string]string{{"email": toEmail, "name": recipientName}},
        Subject:     subject,
        HTMLContent: htmlContent,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payload: %v", err)
    }

    log.Printf("Sending email to %s with payload: %s", toEmail, string(body))

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
    if err != nil {
        return fmt.Errorf("failed to create request: %v", err)
    }

    req.Header.Set("accept", "application/json")
    req.Header.Set("api-key", s.APIKey)
    req.Header.Set("content-type", "application/json")

    client := &http.Client{
        Timeout: 10 * time.Second,
    }
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send request: %v", err)
    }
    defer resp.Body.Close()

    bodyBytes, _ := io.ReadAll(resp.Body)
    if resp.StatusCode != http.StatusCreated {
        log.Printf("Brevo API error: Status %d, Body: %s", resp.StatusCode, string(bodyBytes))
        return fmt.Errorf("failed to send email via Brevo: %s", string(bodyBytes))
    }

    log.Printf("Brevo API response: %s", string(bodyBytes))
    return nil
}

func SendEmail(toName, toEmail, subject, htmlContent string) {
    if EmailClient == nil {
        log.Println("Email client not initialized, skipping email send.")
        return
    }

    log.Printf("Calling SendEmail with toName=%s, toEmail=%s, subject=%s", toName, toEmail, subject)
    err := EmailClient.send(toEmail, toName, subject, htmlContent)
    if err != nil {
        log.Printf("🔥 Failed to send email to %s: %v", toEmail, err)
        return
    }

    log.Printf("✅ Email sent successfully to %s", toEmail)
}