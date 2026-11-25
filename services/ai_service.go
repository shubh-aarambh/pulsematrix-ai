package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"expense-matrix/config"
	"expense-matrix/models"
)

type AIService struct {
	cfg *config.Config
}

func NewAIService(cfg *config.Config) *AIService {
	return &AIService{cfg: cfg}
}

// ParseReceiptOrText extracts vendor, amount, currency, category, and anomaly metadata from text or receipt input.
func (s *AIService) ParseReceiptOrText(rawText string) (*models.AIParsedReceipt, error) {
	if s.cfg.EnableAIServices && s.cfg.GeminiAPIKey != "" {
		parsed, err := s.callGeminiParser(rawText)
		if err == nil && parsed != nil {
			return parsed, nil
		}
	}

	// Intelligent Heuristic Local Parsing Fallback
	return s.parseHeuristically(rawText), nil
}

var (
	rePrefixCurrency = regexp.MustCompile(`(?i)(\$|₹|€|£|Rs\.?|INR|USD|EUR|GBP|CAD|AUD)\s*([0-9][0-9,]*(?:\.[0-9]{1,2})?)`)
	reSuffixCurrency = regexp.MustCompile(`(?i)([0-9][0-9,]*(?:\.[0-9]{1,2})?)\s*(USD|INR|EUR|GBP|CAD|AUD|Rs\.?)`)
	rePlainAmount    = regexp.MustCompile(`([0-9]+(?:\.[0-9]{1,2})?)`)
	reVendorPhrase   = regexp.MustCompile(`(?i)(?:\bto\b|\bat\b|\bfrom\b)\s+([A-Za-z][A-Za-z0-9 &'.-]{1,35}?)(?:\s+(?:via|on|for|using|with|ref|today|yesterday)\b|[.,;]|$)`)
)

func normalizeCurrencySymbol(sym string) string {
	switch strings.ToUpper(strings.TrimSpace(strings.TrimSuffix(sym, "."))) {
	case "₹", "INR", "RS":
		return "INR"
	case "€", "EUR":
		return "EUR"
	case "£", "GBP":
		return "GBP"
	case "CAD":
		return "CAD"
	case "AUD":
		return "AUD"
	default:
		return "USD"
	}
}

func (s *AIService) parseHeuristically(raw string) *models.AIParsedReceipt {
	text := strings.TrimSpace(raw)

	res := &models.AIParsedReceipt{
		Vendor:            "Merchant",
		Amount:            0.0,
		Currency:          "USD",
		SuggestedCategory: "Miscellaneous",
		Date:              time.Now().Format("2006-01-02"),
		PaymentMethod:     "Credit Card",
		ConfidenceScore:   0.85,
		RawSummary:        text,
	}

	// Extract Currency & Amount via Regex (prefix currency, suffix currency, or plain number)
	if m := rePrefixCurrency.FindStringSubmatch(text); len(m) > 2 {
		cleaned := strings.ReplaceAll(m[2], ",", "")
		if val, err := strconv.ParseFloat(cleaned, 64); err == nil && val > 0 {
			res.Amount = val
		}
		res.Currency = normalizeCurrencySymbol(m[1])
	} else if m := reSuffixCurrency.FindStringSubmatch(text); len(m) > 2 {
		cleaned := strings.ReplaceAll(m[1], ",", "")
		if val, err := strconv.ParseFloat(cleaned, 64); err == nil && val > 0 {
			res.Amount = val
		}
		res.Currency = normalizeCurrencySymbol(m[2])
	} else if m := rePlainAmount.FindStringSubmatch(text); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil && val > 0 {
			res.Amount = val
		}
	}

	lower := strings.ToLower(text)

	// Detect Payment Method heuristically
	switch {
	case strings.Contains(lower, "google pay") || strings.Contains(lower, "gpay"):
		res.PaymentMethod = "Google Pay / UPI"
	case strings.Contains(lower, "phonepe"):
		res.PaymentMethod = "PhonePe / UPI"
	case strings.Contains(lower, "paytm"):
		res.PaymentMethod = "Paytm / UPI"
	case strings.Contains(lower, "upi"):
		res.PaymentMethod = "UPI"
	case strings.Contains(lower, "apple pay"):
		res.PaymentMethod = "Apple Pay"
	case strings.Contains(lower, "paypal"):
		res.PaymentMethod = "PayPal"
	case strings.Contains(lower, "debit"):
		res.PaymentMethod = "Debit Card"
	case strings.Contains(lower, "bank") || strings.Contains(lower, "ach") || strings.Contains(lower, "wire"):
		res.PaymentMethod = "Bank Transfer"
	}

	// Extract Vendor & Category heuristic matching
	switch {
	case strings.Contains(lower, "starbucks") || strings.Contains(lower, "coffee") || strings.Contains(lower, "sweetgreen") || strings.Contains(lower, "swiggy") || strings.Contains(lower, "zomato") || strings.Contains(lower, "mcdonald") || strings.Contains(lower, "restaurant") || strings.Contains(lower, "food") || strings.Contains(lower, "dinner") || strings.Contains(lower, "lunch"):
		res.SuggestedCategory = "Food & Dining"
		switch {
		case strings.Contains(lower, "starbucks"):
			res.Vendor = "Starbucks Coffee"
		case strings.Contains(lower, "sweetgreen"):
			res.Vendor = "Sweetgreen"
		case strings.Contains(lower, "swiggy"):
			res.Vendor = "Swiggy"
		case strings.Contains(lower, "zomato"):
			res.Vendor = "Zomato"
		case strings.Contains(lower, "mcdonald"):
			res.Vendor = "McDonald's"
		default:
			res.Vendor = "Local Dining / Restaurant"
		}
	case strings.Contains(lower, "aws") || strings.Contains(lower, "amazon web services") || strings.Contains(lower, "cloud") || strings.Contains(lower, "openai") || strings.Contains(lower, "github"):
		res.SuggestedCategory = "Technology & Cloud"
		switch {
		case strings.Contains(lower, "aws") || strings.Contains(lower, "amazon web services"):
			res.Vendor = "Amazon Web Services"
		case strings.Contains(lower, "openai"):
			res.Vendor = "OpenAI"
		case strings.Contains(lower, "github"):
			res.Vendor = "GitHub"
		default:
			res.Vendor = "Cloud Infrastructure"
		}
	case strings.Contains(lower, "netflix") || strings.Contains(lower, "spotify") || strings.Contains(lower, "youtube") || strings.Contains(lower, "hulu"):
		res.SuggestedCategory = "Subscriptions & SaaS"
		switch {
		case strings.Contains(lower, "netflix"):
			res.Vendor = "Netflix"
		case strings.Contains(lower, "spotify"):
			res.Vendor = "Spotify"
		case strings.Contains(lower, "youtube"):
			res.Vendor = "YouTube Premium"
		case strings.Contains(lower, "hulu"):
			res.Vendor = "Hulu"
		}
	case strings.Contains(lower, "uber") || strings.Contains(lower, "lyft") || strings.Contains(lower, "ola") || strings.Contains(lower, "airline") || strings.Contains(lower, "flight") || strings.Contains(lower, "hotel"):
		res.SuggestedCategory = "Travel & Transport"
		switch {
		case strings.Contains(lower, "uber"):
			res.Vendor = "Uber"
		case strings.Contains(lower, "lyft"):
			res.Vendor = "Lyft"
		case strings.Contains(lower, "ola"):
			res.Vendor = "Ola Cabs"
		default:
			res.Vendor = "Travel & Transit Services"
		}
	case strings.Contains(lower, "rent") || strings.Contains(lower, "apartment") || strings.Contains(lower, "utility") || strings.Contains(lower, "electric"):
		res.SuggestedCategory = "Housing & Utilities"
		res.Vendor = "Property & Housing Mgmt"
	case strings.Contains(lower, "amazon") || strings.Contains(lower, "apple") || strings.Contains(lower, "flipkart") || strings.Contains(lower, "store") || strings.Contains(lower, "target") || strings.Contains(lower, "walmart"):
		res.SuggestedCategory = "Shopping & Retail"
		switch {
		case strings.Contains(lower, "amazon"):
			res.Vendor = "Amazon"
		case strings.Contains(lower, "apple"):
			res.Vendor = "Apple Store"
		case strings.Contains(lower, "flipkart"):
			res.Vendor = "Flipkart"
		case strings.Contains(lower, "target"):
			res.Vendor = "Target"
		case strings.Contains(lower, "walmart"):
			res.Vendor = "Walmart"
		default:
			res.Vendor = "Retail Merchant"
		}
	case strings.Contains(lower, "salary") || strings.Contains(lower, "payroll") || strings.Contains(lower, "deposit") || strings.Contains(lower, "freelance"):
		res.SuggestedCategory = "Salary & Income"
		res.Vendor = "Income Deposit"
	default:
		if m := reVendorPhrase.FindStringSubmatch(text); len(m) > 1 {
			extracted := strings.TrimSpace(m[1])
			if extracted != "" {
				res.Vendor = extracted
			}
		}
	}

	// Anomaly detection check (threshold adjusted for INR vs USD-like currencies)
	threshold := 1000.0
	if res.Currency == "INR" {
		threshold = 50000.0
	} else if res.Currency == "JPY" {
		threshold = 100000.0
	}
	if res.Amount > threshold {
		res.IsAnomaly = true
		res.AnomalyReason = fmt.Sprintf("High-value single transaction (%.2f %s)", res.Amount, res.Currency)
	}

	return res
}

func (s *AIService) callGeminiParser(rawText string) (*models.AIParsedReceipt, error) {
	model := s.cfg.GeminiModel
	if model == "" {
		model = "gemini-2.0-flash"
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, s.cfg.GeminiAPIKey)

	prompt := fmt.Sprintf(`You are a financial receipt parser. Extract data from this raw receipt/expense text into a valid JSON object matching this schema:
{
  "vendor": "String",
  "amount": Float,
  "currency": "USD/EUR/INR/GBP",
  "suggestedCategory": "Salary & Income|Housing & Utilities|Food & Dining|Technology & Cloud|Travel & Transport|Shopping & Retail|Subscriptions & SaaS|Investments & Crypto|Miscellaneous",
  "date": "YYYY-MM-DD",
  "paymentMethod": "Credit Card|Debit Card|Apple Pay|UPI|Bank Transfer",
  "confidenceScore": Float (0.0 to 1.0),
  "isAnomaly": Boolean,
  "anomalyReason": "String",
  "rawSummary": "Brief summary"
}

Raw Text:
"%s"

Return ONLY valid JSON. Do not include markdown code block syntax.`, rawText)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
	})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini api returned status: %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil || len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("invalid response structure from gemini")
	}

	jsonText := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	jsonText = strings.TrimPrefix(jsonText, "```json")
	jsonText = strings.TrimPrefix(jsonText, "```")
	jsonText = strings.TrimSuffix(jsonText, "```")
	jsonText = strings.TrimSpace(jsonText)

	var result models.AIParsedReceipt
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return nil, err
	}

	if result.Date == "" {
		result.Date = time.Now().Format("2006-01-02")
	}
	if result.Currency == "" {
		result.Currency = "USD"
	}

	return &result, nil
}

// GenerateFinancialAdvice provides AI-driven financial counseling & answer queries.
func (s *AIService) GenerateFinancialAdvice(contextSummary string, userPrompt string) (string, error) {
	if s.cfg.EnableAIServices && s.cfg.GeminiAPIKey != "" {
		advice, err := s.callGeminiAdvisor(contextSummary, userPrompt)
		if err == nil && advice != "" {
			return advice, nil
		}
	}

	// Context-Aware Heuristic Advisor Fallback
	prompt := strings.ToLower(userPrompt)
	switch {
	case strings.Contains(prompt, "subscription") || strings.Contains(prompt, "cut") || strings.Contains(prompt, "save"):
		return fmt.Sprintf("💡 **AI Subscription Optimizer Advice**:\n- Live Snapshot: %s\n- **Recommendation**: Audit recurring SaaS and cloud services in your Subscriptions tab. Rightsizing idle cloud instances and consolidating overlapping streaming tiers typically trims 15–25%% of recurring burn.\n- Set monthly renewal reminders 3 days before billing dates to prevent unwanted auto-renewals.", contextSummary), nil
	case strings.Contains(prompt, "food") || strings.Contains(prompt, "dining") || strings.Contains(prompt, "restaurant"):
		return fmt.Sprintf("🍜 **Food & Dining Spend Audit**:\n- Live Snapshot: %s\n- **Insight**: Discretionary dining surges often come from a small number of high-ticket meals or delivery markups.\n- **Action Plan**: Cap weekday delivery orders and track high-value dining charges in the Transactions tab to keep Food & Dining under target.", contextSummary), nil
	case strings.Contains(prompt, "budget") || strings.Contains(prompt, "health") || strings.Contains(prompt, "score") || strings.Contains(prompt, "risk"):
		return fmt.Sprintf("📈 **Financial Health & Risk Audit**:\n- **Live Metrics**: %s\n- **Next Actions**:\n1. Review any flagged anomaly alerts in your Ledger for duplicate charges.\n2. Keep a 6-month emergency liquidity buffer before allocating surplus capital to automated index fund DCA.", contextSummary), nil
	default:
		return fmt.Sprintf("📊 **PulseMatrix AI Portfolio Insight**:\n- **Current Ledger State**: %s\n- Review your **Analytics** velocity chart to track month-over-month net savings, and check the **Transactions** tab to verify any anomaly flags.\n- Tip: Configure `GEMINI_API_KEY` in your environment for full Gemini LLM conversational reasoning.", contextSummary), nil
	}
}

func (s *AIService) callGeminiAdvisor(contextSummary string, userPrompt string) (string, error) {
	model := s.cfg.GeminiModel
	if model == "" {
		model = "gemini-2.0-flash"
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, s.cfg.GeminiAPIKey)

	sysPrompt := fmt.Sprintf(`You are PulseMatrix AI, an expert quantitative financial advisor and wealth strategist.
Give concise, highly actionable, numerical advice based on the user's financial context below.

Financial Context:
%s

User Question:
"%s"`, contextSummary, userPrompt)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": sysPrompt},
				},
			},
		},
	})

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini api error: %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil || len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("invalid response structure")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}
