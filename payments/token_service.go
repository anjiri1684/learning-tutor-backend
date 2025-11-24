package payments

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	config "github.com/anjiri1684/language_tutor/configs"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` 
}

var (
	kcbToken      string
	kcbTokenExpiry time.Time
	tokenMutex    sync.RWMutex
)

const kcbTokenURL = "https://api.buni.kcbgroup.com/token?grant_type=client_credentials"


func GetKcbAccessToken() (string, error) {
	tokenMutex.RLock()
	if kcbToken != "" && time.Now().Before(kcbTokenExpiry) {
		token := kcbToken
		tokenMutex.RUnlock()
		return token, nil
	}
	tokenMutex.RUnlock()

	tokenMutex.Lock()
	defer tokenMutex.Unlock()

	if kcbToken != "" && time.Now().Before(kcbTokenExpiry) {
		return kcbToken, nil
	}

	log.Println("Fetching new KCB access token...")
	apiKey := config.Config("KCB_API_KEY")
	apiSecret := config.Config("KCB_API_SECRET")

	reqBody := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequest("POST", kcbTokenURL, reqBody)
	if err != nil { return "", err }

	req.SetBasicAuth(apiKey, apiSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	fmt.Println(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("KCB token API returned non-200 status: %s", resp.Status)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	kcbToken = tokenResp.AccessToken
	kcbTokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-300) * time.Second)
	log.Println("Successfully fetched and cached KCB access token.")
	
	return kcbToken, nil
}