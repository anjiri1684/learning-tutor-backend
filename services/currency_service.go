package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	config "github.com/anjiri1684/language_tutor/configs"
)

type ExchangeRateResponse struct {
	Result         string             `json:"result"`
	ConversionRates map[string]float64 `json:"conversion_rates"`
}

var (
	ratesCache     map[string]float64
	cacheMutex     sync.RWMutex
	lastFetchTime  time.Time
)

func FetchRates() (map[string]float64, error) {
	cacheMutex.RLock()
	if time.Since(lastFetchTime) < 6*time.Hour && ratesCache != nil {
		cacheMutex.RUnlock()
		return ratesCache, nil
	}
	cacheMutex.RUnlock()

	log.Println("Fetching fresh exchange rates from API...")
	apiKey := config.Config("EXCHANGE_RATE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("exchange rate API key not configured")
	}
	
	url := fmt.Sprintf("https://v6.exchangerate-api.com/v6/%s/latest/USD", apiKey)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data ExchangeRateResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Result != "success" {
		return nil, fmt.Errorf("currency API returned an error")
	}

	cacheMutex.Lock()
	ratesCache = data.ConversionRates
	lastFetchTime = time.Now()
	cacheMutex.Unlock()
	log.Println("Successfully updated currency exchange rate cache.")

	return ratesCache, nil
}

func ConvertUSDToKES(amountUSD float64) (float64, error) {
	rates, err := FetchRates()
	if err != nil {
		return 0, err
	}

	kesRate, ok := rates["KES"]
	if !ok {
		return 0, fmt.Errorf("KES exchange rate not found in API response")
	}

	return amountUSD * kesRate, nil
}