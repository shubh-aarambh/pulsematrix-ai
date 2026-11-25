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
	DB           *db.DB
	AIService    *services.AIService
	IntelService *services.IntelligenceService
	Templates    *template.Template
}

func NewAIHandler(database *db.DB, ai *services.AIService, intel *services.IntelligenceService, tmpl *template.Template) *AIHandler {
	return &AIHandler{
		DB:           database,
		AIService:    ai,
		IntelService: intel,
		Templates:    tmpl,
	}
}

func (h *AIHandler) ShowAdvisor(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, userID, username)

	health, err := h.IntelService.ComputeFinancialHealth(userID)
	if err != nil || health == nil {
		health = &models.FinancialHealth{Score: 85, Rating: "Healthy"}
	}

	viewCtx["Title"] = "AI Financial Assistant - PulseMatrix AI"
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

	contextSummary := fmt.Sprintf(
		"Monthly Income: $%.2f, Monthly Spend: $%.2f, Net Savings: $%.2f, Savings Rate: %.1f%%, Health Score: %d (%s), Top Category: %s, Active Anomalies: %d",
		health.MonthlyIncome, health.MonthlySpend, health.NetSavings, health.SavingsRate, health.Score, health.Rating, health.TopCategory, health.AnomalyCount,
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
