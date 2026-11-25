package handlers

import (
	"encoding/json"
	"html/template"
	"math"
	"net/http"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
	"expense-matrix/services"
)

type AnalyticsHandler struct {
	DB              *db.DB
	Templates       *template.Template
	IntelService    *services.IntelligenceService
	CurrencyService *services.CurrencyService
}

func (h *AnalyticsHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, h.CurrencyService, userID, username)

	health, err := h.IntelService.ComputeFinancialHealth(userID)
	if err != nil || health == nil {
		health = &models.FinancialHealth{Score: 85, Rating: "Healthy"}
	}

	viewCtx["Title"] = "Analytics & Risk Radar"
	viewCtx["ActivePage"] = "analytics"
	viewCtx["Health"] = health
	viewCtx["Heatmap"] = h.IntelService.FetchDailyHeatmap(userID, 30)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}

func (h *AnalyticsHandler) userCurrencyRate(userID int64) (string, string, float64) {
	code := "INR"
	_ = h.DB.QueryRow(`SELECT preferred_currency FROM users WHERE id = ?`, userID).Scan(&code)
	if code == "" {
		code = "INR"
	}
	rate := 1.0
	if h.CurrencyService != nil && code != "INR" {
		rate = h.CurrencyService.RateBetween("INR", code)
	}
	return code, services.SymbolFor(code), rate
}

func (h *AnalyticsHandler) TrendsJSON(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	w.Header().Set("Content-Type", "application/json")

	trends, err := h.IntelService.FetchMonthlyTrends(userID)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, "Failed to fetch trend data")
		return
	}

	code, sym, rate := h.userCurrencyRate(userID)
	for i := range trends {
		trends[i].Income = math.Round(trends[i].Income*rate*100) / 100
		trends[i].Expense = math.Round(trends[i].Expense*rate*100) / 100
		trends[i].Savings = math.Round(trends[i].Savings*rate*100) / 100
		trends[i].BudgetPace = math.Round(trends[i].BudgetPace*rate*100) / 100
	}

	w.Header().Set("X-Currency-Code", code)
	w.Header().Set("X-Currency-Symbol", sym)
	_ = json.NewEncoder(w).Encode(trends)
}

func (h *AnalyticsHandler) CategoriesJSON(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	w.Header().Set("Content-Type", "application/json")

	breakdown, err := h.IntelService.FetchCategoryBreakdown(userID)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, "Failed to fetch category breakdown")
		return
	}

	code, sym, rate := h.userCurrencyRate(userID)
	for i := range breakdown {
		breakdown[i].Amount = math.Round(breakdown[i].Amount*rate*100) / 100
	}

	w.Header().Set("X-Currency-Code", code)
	w.Header().Set("X-Currency-Symbol", sym)
	_ = json.NewEncoder(w).Encode(breakdown)
}
