package handlers

import (
	"html/template"
	"net/http"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
	"expense-matrix/services"
)

type DashboardHandler struct {
	DB           *db.DB
	Templates    *template.Template
	IntelService *services.IntelligenceService
}

func NewDashboardHandler(database *db.DB, tmpl *template.Template, intel *services.IntelligenceService) *DashboardHandler {
	return &DashboardHandler{
		DB:           database,
		Templates:    tmpl,
		IntelService: intel,
	}
}

func (h *DashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, userID, username)

	health, err := h.IntelService.ComputeFinancialHealth(userID)
	if err != nil || health == nil {
		health = &models.FinancialHealth{Score: 85, Rating: "Healthy"}
	}

	rows, err := h.DB.Query(`
		SELECT t.id, t.amount, t.original_currency, t.vendor, t.description, t.date, t.source, t.is_anomaly, t.anomaly_reason, COALESCE(c.name, 'Miscellaneous'), COALESCE(c.icon, '📦'), COALESCE(c.color, '#6366f1')
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = ?
		ORDER BY t.date DESC, t.id DESC LIMIT 6`, userID)

	type TxView struct {
		ID            int64
		Amount        float64
		Currency      string
		Vendor        string
		Description   string
		Date          string
		Source        string
		IsAnomaly     bool
		Reason        string
		Category      string
		CategoryIcon  string
		CategoryColor string
	}

	var recentTxs []TxView
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tx TxView
			var isAnom int
			if err := rows.Scan(&tx.ID, &tx.Amount, &tx.Currency, &tx.Vendor, &tx.Description, &tx.Date, &tx.Source, &isAnom, &tx.Reason, &tx.Category, &tx.CategoryIcon, &tx.CategoryColor); err == nil {
				tx.IsAnomaly = isAnom == 1
				recentTxs = append(recentTxs, tx)
			}
		}
	}

	cats, _ := h.IntelService.FetchCategoryBreakdown(userID)

	var activeSubCount int
	var activeSubTotal float64
	_ = h.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(
			CASE
				WHEN billing_cycle = 'yearly' THEN amount / 12.0
				WHEN billing_cycle = 'weekly' THEN amount * 4.333
				ELSE amount
			END
		), 0)
		FROM subscriptions
		WHERE user_id = ? AND status = 'active'`, userID).Scan(&activeSubCount, &activeSubTotal)

	viewCtx["Title"] = "Command Center - PulseMatrix AI"
	viewCtx["ActivePage"] = "dashboard"
	viewCtx["Health"] = health
	viewCtx["RecentTxs"] = recentTxs
	viewCtx["CategorySpends"] = cats
	viewCtx["ActiveSubCount"] = activeSubCount
	viewCtx["ActiveSubTotal"] = activeSubTotal
	viewCtx["FlashSuccess"] = r.URL.Query().Get("success")
	viewCtx["Error"] = r.URL.Query().Get("error")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}
