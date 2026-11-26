package handlers

import (
	"html/template"
	"math"
	"net/http"
	"time"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
	"expense-matrix/services"
)

type ReportHandler struct {
	DB              *db.DB
	Templates       *template.Template
	IntelService    *services.IntelligenceService
	CurrencyService *services.CurrencyService
}

func NewReportHandler(database *db.DB, tmpl *template.Template, intel *services.IntelligenceService, cs *services.CurrencyService) *ReportHandler {
	return &ReportHandler{DB: database, Templates: tmpl, IntelService: intel, CurrencyService: cs}
}

func (h *ReportHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, h.CurrencyService, userID, username)

	health, err := h.IntelService.ComputeFinancialHealth(userID)
	if err != nil || health == nil {
		health = &models.FinancialHealth{Score: 85, Rating: "Healthy"}
	}
	cats, _ := h.IntelService.FetchCategoryBreakdown(userID)

	var totalAssets, totalLiabilities float64
	_ = h.DB.QueryRow(`SELECT COALESCE(SUM(value), 0) FROM net_worth_items WHERE user_id = ? AND kind = 'asset'`, userID).Scan(&totalAssets)
	_ = h.DB.QueryRow(`SELECT COALESCE(SUM(value), 0) FROM net_worth_items WHERE user_id = ? AND kind = 'liability'`, userID).Scan(&totalLiabilities)

	var owedToMe, iOwe float64
	_ = h.DB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM split_expenses WHERE user_id = ? AND status = 'pending' AND direction = 'owed_to_me'`, userID).Scan(&owedToMe)
	_ = h.DB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM split_expenses WHERE user_id = ? AND status = 'pending' AND direction = 'i_owe'`, userID).Scan(&iOwe)

	var goals []models.SavingsGoal
	if gRows, err := h.DB.Query(`SELECT id, name, icon, target_amount, current_amount, target_date, status FROM savings_goals WHERE user_id = ? ORDER BY target_date ASC`, userID); err == nil {
		defer gRows.Close()
		for gRows.Next() {
			var g models.SavingsGoal
			if err := gRows.Scan(&g.ID, &g.Name, &g.Icon, &g.TargetAmount, &g.CurrentAmount, &g.TargetDate, &g.Status); err == nil {
				if g.TargetAmount > 0 {
					g.ProgressPct = math.Round((g.CurrentAmount/g.TargetAmount)*1000) / 10
				}
				goals = append(goals, g)
			}
		}
	}

	var subs []models.Subscription
	if sRows, err := h.DB.Query(`SELECT id, name, amount, billing_cycle, next_billing_date, status FROM subscriptions WHERE user_id = ? AND status = 'active' ORDER BY amount DESC`, userID); err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var s models.Subscription
			if err := sRows.Scan(&s.ID, &s.Name, &s.Amount, &s.BillingCycle, &s.NextBillingDate, &s.Status); err == nil {
				subs = append(subs, s)
			}
		}
	}

	viewCtx["Title"] = "Monthly Executive Statement"
	viewCtx["ActivePage"] = "report"
	viewCtx["StatementMonth"] = time.Now().Format("January 2006")
	viewCtx["GeneratedAt"] = time.Now().Format("02 Jan 2006 • 15:04 IST")
	viewCtx["Health"] = health
	viewCtx["CategorySpends"] = cats
	viewCtx["TotalAssets"] = totalAssets
	viewCtx["TotalLiabilities"] = totalLiabilities
	viewCtx["NetWorth"] = totalAssets - totalLiabilities
	viewCtx["OwedToMe"] = owedToMe
	viewCtx["IOwe"] = iOwe
	viewCtx["Goals"] = goals
	viewCtx["Subscriptions"] = subs

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}
