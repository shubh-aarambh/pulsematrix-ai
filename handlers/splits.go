package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
	"expense-matrix/services"
)

type SplitsHandler struct {
	DB              *db.DB
	Templates       *template.Template
	CurrencyService *services.CurrencyService
}

func NewSplitsHandler(database *db.DB, tmpl *template.Template, cs *services.CurrencyService) *SplitsHandler {
	return &SplitsHandler{DB: database, Templates: tmpl, CurrencyService: cs}
}

func (h *SplitsHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, h.CurrencyService, userID, username)

	rows, err := h.DB.Query(`
		SELECT id, person_name, description, amount, currency, direction, status, due_date, strftime('%Y-%m-%d', created_at)
		FROM split_expenses
		WHERE user_id = ?
		ORDER BY CASE WHEN status = 'pending' THEN 0 ELSE 1 END, due_date ASC, id DESC`, userID)

	var splits []models.SplitExpense
	var owedToMe, iOwe float64
	var pendingCount int

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sp models.SplitExpense
			if err := rows.Scan(&sp.ID, &sp.PersonName, &sp.Description, &sp.Amount, &sp.Currency, &sp.Direction, &sp.Status, &sp.DueDate, &sp.CreatedAt); err == nil {
				splits = append(splits, sp)
				if sp.Status == "pending" {
					pendingCount++
					if sp.Direction == "owed_to_me" {
						owedToMe += sp.Amount
					} else {
						iOwe += sp.Amount
					}
				}
			}
		}
	}

	viewCtx["Title"] = "Split-Bill & Shared Ledger"
	viewCtx["ActivePage"] = "splits"
	viewCtx["Splits"] = splits
	viewCtx["OwedToMe"] = owedToMe
	viewCtx["IOwe"] = iOwe
	viewCtx["NetSplitBalance"] = owedToMe - iOwe
	viewCtx["PendingCount"] = pendingCount
	viewCtx["FlashSuccess"] = r.URL.Query().Get("success")
	viewCtx["Error"] = r.URL.Query().Get("error")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}

func (h *SplitsHandler) Add(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)

	person := strings.TrimSpace(r.FormValue("person_name"))
	desc := strings.TrimSpace(r.FormValue("description"))
	amt, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("amount")), 64)
	curr := strings.ToUpper(strings.TrimSpace(r.FormValue("currency")))
	dir := strings.TrimSpace(r.FormValue("direction"))
	due := strings.TrimSpace(r.FormValue("due_date"))

	if person == "" || desc == "" || err != nil || amt <= 0 {
		http.Redirect(w, r, "/splits?error=Please+enter+person,+description,+and+a+positive+amount", http.StatusSeeOther)
		return
	}
	if curr == "" {
		curr = "INR"
	}
	if dir != "owed_to_me" && dir != "i_owe" {
		dir = "owed_to_me"
	}
	if due == "" {
		due = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	}

	baseAmt, _, _ := h.CurrencyService.Convert(amt, curr, "INR")

	_, dbErr := h.DB.Exec(`
		INSERT INTO split_expenses (user_id, person_name, description, amount, currency, direction, status, due_date)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', ?)`,
		userID, person, desc, baseAmt, curr, dir, due,
	)
	if dbErr != nil {
		http.Redirect(w, r, "/splits?error=Failed+to+record+split+expense", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/splits?success=Shared+expense+added", http.StatusSeeOther)
}

func (h *SplitsHandler) Settle(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	splitID, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil || splitID <= 0 {
		http.Redirect(w, r, "/splits?error=Invalid+split+ID", http.StatusSeeOther)
		return
	}

	var person, desc, dir string
	var amt float64
	if err := h.DB.QueryRow(`SELECT person_name, description, amount, direction FROM split_expenses WHERE id = ? AND user_id = ? AND status = 'pending'`, splitID, userID).Scan(&person, &desc, &amt, &dir); err == nil {
		_, _ = h.DB.Exec(`UPDATE split_expenses SET status = 'settled' WHERE id = ? AND user_id = ?`, splitID, userID)

		// Also log settlement in main transactions ledger if requested
		if r.FormValue("log_ledger") == "1" {
			catName := "Miscellaneous"
			if dir == "owed_to_me" {
				catName = "Salary & Income"
			}
			var catID int64
			_ = h.DB.QueryRow(`SELECT id FROM categories WHERE user_id = ? AND name = ?`, userID, catName).Scan(&catID)
			var catPtr *int64
			if catID > 0 {
				catPtr = &catID
			}
			_, _ = h.DB.Exec(`
				INSERT INTO transactions (user_id, category_id, amount, original_amount, original_currency, exchange_rate, vendor, description, date, source, payment_method)
				VALUES (?, ?, ?, ?, 'INR', 1.0, ?, ?, ?, 'split_settle', 'UPI')`,
				userID, catPtr, amt, amt, person, fmt.Sprintf("Split Settled: %s", desc), time.Now().Format("2006-01-02"),
			)
		}
	}

	http.Redirect(w, r, "/splits?success=Split+marked+as+settled", http.StatusSeeOther)
}

func (h *SplitsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	_, _ = h.DB.Exec(`DELETE FROM split_expenses WHERE id = ? AND user_id = ?`, r.FormValue("id"), userID)
	http.Redirect(w, r, "/splits?success=Split+entry+removed", http.StatusSeeOther)
}
