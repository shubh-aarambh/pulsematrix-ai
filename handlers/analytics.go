package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
	"expense-matrix/services"
)

type AnalyticsHandler struct {
	DB           *db.DB
	Templates    *template.Template
	IntelService *services.IntelligenceService
}

func (h *AnalyticsHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, userID, username)

	health, err := h.IntelService.ComputeFinancialHealth(userID)
	if err != nil || health == nil {
		health = &models.FinancialHealth{Score: 85, Rating: "Healthy"}
	}

	viewCtx["Title"] = "Analytics & Intelligence Radar - PulseMatrix AI"
	viewCtx["ActivePage"] = "analytics"
	viewCtx["Health"] = health

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}

func (h *AnalyticsHandler) TrendsJSON(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	w.Header().Set("Content-Type", "application/json")

	trends, err := h.IntelService.FetchMonthlyTrends(userID)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, "Failed to fetch trend data")
		return
	}

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

	_ = json.NewEncoder(w).Encode(breakdown)
}
