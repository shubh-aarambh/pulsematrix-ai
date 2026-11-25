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

type DashboardHandler struct {
	DB              *db.DB
	Templates       *template.Template
	IntelService    *services.IntelligenceService
	CurrencyService *services.CurrencyService
}

func NewDashboardHandler(database *db.DB, tmpl *template.Template, intel *services.IntelligenceService, cs *services.CurrencyService) *DashboardHandler {
	return &DashboardHandler{
		DB:              database,
		Templates:       tmpl,
		IntelService:    intel,
		CurrencyService: cs,
	}
}

func (h *DashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, h.CurrencyService, userID, username)

	health, err := h.IntelService.ComputeFinancialHealth(userID)
	if err != nil || health == nil {
		health = &models.FinancialHealth{Score: 85, Rating: "Healthy"}
	}

	rows, err := h.DB.Query(`
		SELECT t.id, t.amount, t.original_currency, t.vendor, t.description, t.date, t.source, t.is_anomaly, t.anomaly_reason, COALESCE(c.name, 'Miscellaneous'), COALESCE(c.icon, '📦'), COALESCE(c.color, '#10b981')
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

	// Top 3 active savings goals preview
	var topGoals []models.SavingsGoal
	if gRows, err := h.DB.Query(`
		SELECT id, name, icon, target_amount, current_amount, target_date, status
		FROM savings_goals
		WHERE user_id = ?
		ORDER BY CASE WHEN status = 'active' THEN 0 ELSE 1 END, target_date ASC LIMIT 3`, userID); err == nil {
		defer gRows.Close()
		for gRows.Next() {
			var g models.SavingsGoal
			if err := gRows.Scan(&g.ID, &g.Name, &g.Icon, &g.TargetAmount, &g.CurrentAmount, &g.TargetDate, &g.Status); err == nil {
				if g.TargetAmount > 0 {
					g.ProgressPct = math.Round((g.CurrentAmount/g.TargetAmount)*1000) / 10
				}
				topGoals = append(topGoals, g)
			}
		}
	}

	// Upcoming 3 active subscriptions with days countdown
	type RenewalItem struct {
		Name     string
		Amount   float64
		Date     string
		DaysLeft int
	}
	var upcomingRenewals []RenewalItem
	nowDate := time.Now().Truncate(24 * time.Hour)
	if sRows, err := h.DB.Query(`
		SELECT name, amount, next_billing_date
		FROM subscriptions
		WHERE user_id = ? AND status = 'active'
		ORDER BY next_billing_date ASC LIMIT 3`, userID); err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var rItem RenewalItem
			if err := sRows.Scan(&rItem.Name, &rItem.Amount, &rItem.Date); err == nil {
				if parsed, err := time.Parse("2006-01-02", rItem.Date); err == nil {
					diff := int(math.Ceil(parsed.Sub(nowDate).Hours() / 24.0))
					if diff < 0 {
						diff = 0
					}
					rItem.DaysLeft = diff
				}
				upcomingRenewals = append(upcomingRenewals, rItem)
			}
		}
	}

	heatmap := h.IntelService.FetchDailyHeatmap(userID, 30)
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

	viewCtx["Title"] = "Command Center"
	viewCtx["ActivePage"] = "dashboard"
	viewCtx["Health"] = health
	viewCtx["RecentTxs"] = recentTxs
	viewCtx["TopGoals"] = topGoals
	viewCtx["UpcomingRenewals"] = upcomingRenewals
	viewCtx["Heatmap"] = heatmap
	viewCtx["CategorySpends"] = cats
	viewCtx["ActiveSubCount"] = activeSubCount
	viewCtx["ActiveSubTotal"] = activeSubTotal
	viewCtx["FlashSuccess"] = r.URL.Query().Get("success")
	viewCtx["Error"] = r.URL.Query().Get("error")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}
