package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"expense-matrix/config"
)

type CurrencyService struct {
	cfg        *config.Config
	ratesCache map[string]float64
	lastFetch  time.Time
	mu         sync.RWMutex
}

func NewCurrencyService(cfg *config.Config) *CurrencyService {
	cs := &CurrencyService{
		cfg:        cfg,
		ratesCache: make(map[string]float64),
	}
	cs.initDefaultRates()
	return cs
}

func (cs *CurrencyService) initDefaultRates() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Default fallback exchange rates (Base USD)
	cs.ratesCache = map[string]float64{
		"USD": 1.00,
		"EUR": 0.92,
		"GBP": 0.79,
		"INR": 83.50,
		"CAD": 1.36,
		"AUD": 1.52,
		"JPY": 155.20,
	}
	// Leave lastFetch zero so the first conversion refreshes live rates once.
}

// Convert converts an amount from source currency to target currency. Returns (convertedAmount, exchangeRate, error)
func (cs *CurrencyService) Convert(amount float64, from string, to string) (float64, float64, error) {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))

	if from == "" {
		from = "USD"
	}
	if to == "" {
		to = "USD"
	}

	if from == to {
		return amount, 1.0, nil
	}

	rates := cs.GetRates()

	fromRate, ok1 := rates[from]
	toRate, ok2 := rates[to]

	if !ok1 || !ok2 || fromRate <= 0 {
		return amount, 1.0, fmt.Errorf("unsupported currency conversion: %s to %s", from, to)
	}

	rate := toRate / fromRate
	converted := amount * rate
	return converted, rate, nil
}

// GetRates fetches latest rates, using cache if refreshed within 1 hour.
func (cs *CurrencyService) GetRates() map[string]float64 {
	cs.mu.RLock()
	if !cs.lastFetch.IsZero() && time.Since(cs.lastFetch) < 1*time.Hour && len(cs.ratesCache) > 0 {
		ratesCopy := copyRatesMap(cs.ratesCache)
		cs.mu.RUnlock()
		return ratesCopy
	}
	cs.mu.RUnlock()

	// Attempt API refresh
	rates, err := cs.fetchLiveRates()
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.lastFetch = time.Now()
	if err == nil && len(rates) > 0 {
		cs.ratesCache = rates
	}
	return copyRatesMap(cs.ratesCache)
}

func copyRatesMap(src map[string]float64) map[string]float64 {
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (cs *CurrencyService) fetchLiveRates() (map[string]float64, error) {
	url := "https://open.er-api.com/v6/latest/USD"
	if cs.cfg.ExchangeRateAPIKey != "" {
		url = fmt.Sprintf("https://v6.exchangerate-api.com/v6/%s/latest/USD", cs.cfg.ExchangeRateAPIKey)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exchange API error code %d", resp.StatusCode)
	}

	var data struct {
		Rates           map[string]float64 `json:"rates"`
		ConversionRates map[string]float64 `json:"conversion_rates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if len(data.Rates) > 0 {
		return data.Rates, nil
	}
	if len(data.ConversionRates) > 0 {
		return data.ConversionRates, nil
	}

	return nil, fmt.Errorf("no rates found in response")
}
