package handlers

import (
	"encoding/csv"
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

type TransactionHandler struct {
	DB              *db.DB
	Templates       *template.Template
	CurrencyService *services.CurrencyService
}

func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, userID, username)

	search := strings.TrimSpace(r.URL.Query().Get("q"))
	catFilter := strings.TrimSpace(r.URL.Query().Get("category"))

	query := `
		SELECT t.id, t.amount, t.original_amount, t.original_currency, t.exchange_rate, t.vendor, t.description, t.date, t.source, t.payment_method, t.is_recurring, t.is_anomaly, t.anomaly_reason, COALESCE(c.name, 'Miscellaneous'), COALESCE(c.icon, '📦'), COALESCE(c.color, '#64748b')
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = ?`

	args := []interface{}{userID}

	if search != "" {
		query += ` AND (t.vendor LIKE ? OR t.description LIKE ?)`
		sPattern := "%" + search + "%"
		args = append(args, sPattern, sPattern)
	}

	if catFilter != "" {
		query += ` AND COALESCE(c.name, 'Miscellaneous') = ?`
		args = append(args, catFilter)
	}

	query += ` ORDER BY t.date DESC, t.id DESC`

	rows, err := h.DB.Query(query, args...)
	var transactions []models.Transaction
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tx models.Transaction
			var isRec, isAnom int
			if err := rows.Scan(
				&tx.ID, &tx.Amount, &tx.OriginalAmount, &tx.OriginalCurrency, &tx.ExchangeRate,
				&tx.Vendor, &tx.Description, &tx.Date, &tx.Source, &tx.PaymentMethod,
				&isRec, &isAnom, &tx.AnomalyReason, &tx.CategoryName, &tx.CategoryIcon, &tx.CategoryColor,
			); err == nil {
				tx.IsRecurring = isRec == 1
				tx.IsAnomaly = isAnom == 1
				transactions = append(transactions, tx)
			}
		}
	}

	viewCtx["Title"] = "Ledger & Transactions - PulseMatrix AI"
	viewCtx["ActivePage"] = "transactions"
	viewCtx["Transactions"] = transactions
	viewCtx["Search"] = search
	viewCtx["CatFilter"] = catFilter
	viewCtx["FlashSuccess"] = r.URL.Query().Get("success")
	viewCtx["Error"] = r.URL.Query().Get("error")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}

func (h *TransactionHandler) Add(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)

	vendor := strings.TrimSpace(r.FormValue("vendor"))
	origAmtStr := strings.TrimSpace(r.FormValue("amount"))
	currency := strings.ToUpper(strings.TrimSpace(r.FormValue("currency")))
	catIDStr := strings.TrimSpace(r.FormValue("category_id"))
	catName := strings.TrimSpace(r.FormValue("category_name"))
	txDate := strings.TrimSpace(r.FormValue("date"))
	desc := strings.TrimSpace(r.FormValue("description"))
	method := strings.TrimSpace(r.FormValue("payment_method"))
	source := strings.TrimSpace(r.FormValue("source"))

	origAmount, parseErr := strconv.ParseFloat(origAmtStr, 64)
	if vendor == "" || parseErr != nil || origAmount <= 0 {
		http.Redirect(w, r, "/transactions?error=Please+enter+a+valid+vendor+and+positive+amount", http.StatusSeeOther)
		return
	}

	if txDate == "" {
		txDate = time.Now().Format("2006-01-02")
	}
	if currency == "" {
		currency = "USD"
	}
	if method == "" {
		method = "Credit Card"
	}
	if source == "" {
		source = "manual"
	}

	// Convert currency to base USD for storage normalization
	baseAmount, rate, _ := h.CurrencyService.Convert(origAmount, currency, "USD")

	var catID *int64
	if catIDStr != "" {
		if cid, err := strconv.ParseInt(catIDStr, 10, 64); err == nil && cid > 0 {
			var verifiedID int64
			if err := h.DB.QueryRow(`SELECT id FROM categories WHERE id = ? AND user_id = ?`, cid, userID).Scan(&verifiedID); err == nil {
				catID = &verifiedID
			}
		}
	}
	// Fallback lookup by category name (used by AI Receipt Parser)
	if catID == nil && catName != "" {
		var matchedID int64
		if err := h.DB.QueryRow(`SELECT id FROM categories WHERE user_id = ? AND name = ?`, userID, catName).Scan(&matchedID); err == nil && matchedID > 0 {
			catID = &matchedID
		}
	}

	isAnomaly := 0
	anomalyReason := ""
	if baseAmount > 800.0 {
		isAnomaly = 1
		anomalyReason = fmt.Sprintf("High magnitude charge ($%.2f USD equivalent)", baseAmount)
	}

	_, err := h.DB.Exec(`
		INSERT INTO transactions (
			user_id, category_id, amount, original_amount, original_currency, exchange_rate,
			vendor, description, date, source, payment_method, is_recurring, is_anomaly, anomaly_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		userID, catID, baseAmount, origAmount, currency, rate,
		vendor, desc, txDate, source, method, isAnomaly, anomalyReason,
	)

	if err != nil {
		http.Redirect(w, r, "/transactions?error=Failed+to+save+transaction", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/transactions?success=Transaction+recorded", http.StatusSeeOther)
}

func (h *TransactionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	txID := r.FormValue("id")

	_, _ = h.DB.Exec(`DELETE FROM transactions WHERE id = ? AND user_id = ?`, txID, userID)
	http.Redirect(w, r, "/transactions?success=Transaction+deleted", http.StatusSeeOther)
}

func (h *TransactionHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)

	rows, err := h.DB.Query(`
		SELECT t.date, t.vendor, t.amount, t.original_amount, t.original_currency, t.payment_method, COALESCE(c.name, 'Uncategorized'), t.source, t.description
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = ?
		ORDER BY t.date DESC`, userID)
	if err != nil {
		http.Error(w, "Failed to generate CSV export", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=pulsematrix_ledger_%s.csv", time.Now().Format("20060102")))

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"Date", "Vendor", "Amount (USD)", "Original Amount", "Currency", "Payment Method", "Category", "Source", "Description"})

	for rows.Next() {
		var date, vendor, curr, method, cat, source, desc string
		var amt, origAmt float64
		if err := rows.Scan(&date, &vendor, &amt, &origAmt, &curr, &method, &cat, &source, &desc); err == nil {
			_ = writer.Write([]string{date, vendor, fmt.Sprintf("%.2f", amt), fmt.Sprintf("%.2f", origAmt), curr, method, cat, source, desc})
		}
	}

	writer.Flush()
}
