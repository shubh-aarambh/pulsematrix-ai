package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
	"expense-matrix/services"
)

type AIHandler struct {
	DB              *db.DB
	AIService       *services.AIService
	IntelService    *services.IntelligenceService
	CurrencyService *services.CurrencyService
	Templates       *template.Template
}

func NewAIHandler(database *db.DB, ai *services.AIService, intel *services.IntelligenceService, cs *services.CurrencyService, tmpl *template.Template) *AIHandler {
	return &AIHandler{
		DB:              database,
		AIService:       ai,
		IntelService:    intel,
		CurrencyService: cs,
		Templates:       tmpl,
	}
}

func (h *AIHandler) ShowAdvisor(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, h.CurrencyService, userID, username)

	health, err := h.IntelService.ComputeFinancialHealth(userID)
	if err != nil || health == nil {
		health = &models.FinancialHealth{Score: 85, Rating: "Healthy"}
	}

	viewCtx["Title"] = "AI Intelligence Strategist"
	viewCtx["ActivePage"] = "ai-advisor"
	viewCtx["Health"] = health

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}

func (h *AIHandler) ParseReceiptAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Text string `json:"text"`
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&req)
	} else {
		req.Text = r.FormValue("rawText")
	}

	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		WriteJSONError(w, http.StatusBadRequest, "Text prompt or receipt payload is empty")
		return
	}

	result, err := h.AIService.ParseReceiptOrText(req.Text)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("AI parser failed: %v", err))
		return
	}

	_ = json.NewEncoder(w).Encode(result)
}

func (h *AIHandler) AdvisorChatAPI(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Query string `json:"query"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Query) == "" {
		WriteJSONError(w, http.StatusBadRequest, "Prompt query is required")
		return
	}

	health, err := h.IntelService.ComputeFinancialHealth(userID)
	if err != nil || health == nil {
		health = &models.FinancialHealth{Score: 85, Rating: "Healthy", TopCategory: "None yet"}
	}

	currCode := "INR"
	_ = h.DB.QueryRow(`SELECT preferred_currency FROM users WHERE id = ?`, userID).Scan(&currCode)
	rate := 1.0
	if h.CurrencyService != nil && currCode != "INR" {
		rate = h.CurrencyService.RateBetween("INR", currCode)
	}
	sym := services.SymbolFor(currCode)

	contextSummary := fmt.Sprintf(
		"Currency: %s, Monthly Income: %s%.2f, Monthly Spend: %s%.2f, Net Savings: %s%.2f, Savings Rate: %.1f%%, Daily Burn: %s%.0f/day, Health Score: %d (%s), Top Category: %s, Active Anomalies: %d",
		currCode, sym, health.MonthlyIncome*rate, sym, health.MonthlySpend*rate, sym, health.NetSavings*rate, health.SavingsRate, sym, health.DailyBurnRate*rate, health.Score, health.Rating, health.TopCategory, health.AnomalyCount,
	)

	reply, err := h.AIService.GenerateFinancialAdvice(contextSummary, req.Query)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Advisor generation failed: %v", err))
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"query": req.Query,
		"reply": reply,
	})
}
