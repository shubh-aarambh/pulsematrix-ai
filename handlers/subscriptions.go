package handlers

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
)

type SubscriptionsHandler struct {
	DB        *db.DB
	Templates *template.Template
}

func (h *SubscriptionsHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, userID, username)

	rows, err := h.DB.Query(`
		SELECT id, name, amount, currency, category_name, billing_cycle, next_billing_date, status, detected_pattern
		FROM subscriptions
		WHERE user_id = ?
		ORDER BY CASE WHEN status = 'active' THEN 0 ELSE 1 END, next_billing_date ASC`, userID)

	var list []models.Subscription
	var totalMonthly float64
	var activeCount int

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var s models.Subscription
			if err := rows.Scan(&s.ID, &s.Name, &s.Amount, &s.Currency, &s.CategoryName, &s.BillingCycle, &s.NextBillingDate, &s.Status, &s.DetectedPattern); err == nil {
				list = append(list, s)
				if s.Status == "active" {
					activeCount++
					switch s.BillingCycle {
					case "yearly":
						totalMonthly += s.Amount / 12.0
					case "weekly":
						totalMonthly += s.Amount * 4.333
					default:
						totalMonthly += s.Amount
					}
				}
			}
		}
	}

	totalYearly := totalMonthly * 12.0

	viewCtx["Title"] = "Subscriptions & Recurring Radar - PulseMatrix AI"
	viewCtx["ActivePage"] = "subscriptions"
	viewCtx["Subscriptions"] = list
	viewCtx["ActiveSubCount"] = activeCount
	viewCtx["TotalMonthly"] = totalMonthly
	viewCtx["TotalYearly"] = totalYearly
	viewCtx["FlashSuccess"] = r.URL.Query().Get("success")
	viewCtx["Error"] = r.URL.Query().Get("error")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}

func (h *SubscriptionsHandler) Add(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)

	name := strings.TrimSpace(r.FormValue("name"))
	amtStr := strings.TrimSpace(r.FormValue("amount"))
	cycle := strings.TrimSpace(r.FormValue("billing_cycle"))
	catName := strings.TrimSpace(r.FormValue("category_name"))
	nextDate := strings.TrimSpace(r.FormValue("next_billing_date"))

	amt, err := strconv.ParseFloat(amtStr, 64)
	if name == "" || err != nil || amt <= 0 {
		http.Redirect(w, r, "/subscriptions?error=Please+provide+a+valid+service+name+and+amount", http.StatusSeeOther)
		return
	}

	if cycle != "monthly" && cycle != "yearly" && cycle != "weekly" {
		cycle = "monthly"
	}
	if catName == "" {
		catName = "Subscriptions & SaaS"
	}
	if nextDate == "" {
		nextDate = time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	}

	_, err = h.DB.Exec(`
		INSERT INTO subscriptions (user_id, name, amount, currency, category_name, billing_cycle, next_billing_date, status, detected_pattern)
		VALUES (?, ?, ?, 'USD', ?, ?, ?, 'active', 'User Configured')`,
		userID, name, amt, catName, cycle, nextDate,
	)
	if err != nil {
		http.Redirect(w, r, "/subscriptions?error=Failed+to+add+subscription", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/subscriptions?success=Subscription+added", http.StatusSeeOther)
}

func (h *SubscriptionsHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	subID := r.FormValue("id")

	_, _ = h.DB.Exec(`UPDATE subscriptions SET status = 'cancelled' WHERE id = ? AND user_id = ?`, subID, userID)
	http.Redirect(w, r, "/subscriptions?success=Subscription+cancelled", http.StatusSeeOther)
}
