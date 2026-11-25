package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
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

	viewCtx := GetBaseContext(h.DB, h.CurrencyService, userID, username)

	search := strings.TrimSpace(r.URL.Query().Get("q"))
	catFilter := strings.TrimSpace(r.URL.Query().Get("category"))
	onlyAnomalies := r.URL.Query().Get("anomaly") == "1"

	query := `
		SELECT t.id, t.category_id, t.amount, t.original_amount, t.original_currency, t.exchange_rate, t.vendor, t.description, t.date, t.source, t.payment_method, t.is_recurring, t.is_anomaly, t.anomaly_reason, COALESCE(c.name, 'Miscellaneous'), COALESCE(c.icon, '📦'), COALESCE(c.color, '#64748b')
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

	if onlyAnomalies {
		query += ` AND t.is_anomaly = 1`
	}

	query += ` ORDER BY t.date DESC, t.id DESC`

	rows, err := h.DB.Query(query, args...)
	var transactions []models.Transaction
	var anomalyCount int
	var totalVolume float64

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tx models.Transaction
			var isRec, isAnom int
			if err := rows.Scan(
				&tx.ID, &tx.CategoryID, &tx.Amount, &tx.OriginalAmount, &tx.OriginalCurrency, &tx.ExchangeRate,
				&tx.Vendor, &tx.Description, &tx.Date, &tx.Source, &tx.PaymentMethod,
				&isRec, &isAnom, &tx.AnomalyReason, &tx.CategoryName, &tx.CategoryIcon, &tx.CategoryColor,
			); err == nil {
				tx.IsRecurring = isRec == 1
				tx.IsAnomaly = isAnom == 1
				if tx.IsAnomaly {
					anomalyCount++
				}
				totalVolume += tx.Amount
				transactions = append(transactions, tx)
			}
		}
	}

	viewCtx["Title"] = "Ledger & Transactions - PulseMatrix AI"
	viewCtx["ActivePage"] = "transactions"
	viewCtx["Transactions"] = transactions
	viewCtx["TxCount"] = len(transactions)
	viewCtx["AnomalyCount"] = anomalyCount
	viewCtx["TotalVolume"] = totalVolume
	viewCtx["Search"] = search
	viewCtx["CatFilter"] = catFilter
	viewCtx["OnlyAnomalies"] = onlyAnomalies
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
		currency = "INR"
	}
	if method == "" {
		method = "UPI"
	}
	if source == "" {
		source = "manual"
	}

	baseAmount, rate, _ := h.CurrencyService.Convert(origAmount, currency, "INR")

	var catID *int64
	if catIDStr != "" {
		if cid, err := strconv.ParseInt(catIDStr, 10, 64); err == nil && cid > 0 {
			var verifiedID int64
			if err := h.DB.QueryRow(`SELECT id FROM categories WHERE id = ? AND user_id = ?`, cid, userID).Scan(&verifiedID); err == nil {
				catID = &verifiedID
			}
		}
	}
	if catID == nil && catName != "" {
		var matchedID int64
		if err := h.DB.QueryRow(`SELECT id FROM categories WHERE user_id = ? AND name = ?`, userID, catName).Scan(&matchedID); err == nil && matchedID > 0 {
			catID = &matchedID
		}
	}

	sysSettings := h.DB.GetSystemSettings()
	isAnomaly := 0
	anomalyReason := ""
	if baseAmount > sysSettings.AnomalyThreshold {
		isAnomaly = 1
		anomalyReason = fmt.Sprintf("High magnitude charge (₹%.2f > ₹%.0f threshold)", baseAmount, sysSettings.AnomalyThreshold)
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

func (h *TransactionHandler) Edit(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)

	txID, err1 := strconv.ParseInt(r.FormValue("id"), 10, 64)
	vendor := strings.TrimSpace(r.FormValue("vendor"))
	origAmount, err2 := strconv.ParseFloat(strings.TrimSpace(r.FormValue("amount")), 64)
	currency := strings.ToUpper(strings.TrimSpace(r.FormValue("currency")))
	catIDStr := strings.TrimSpace(r.FormValue("category_id"))
	txDate := strings.TrimSpace(r.FormValue("date"))
	desc := strings.TrimSpace(r.FormValue("description"))
	method := strings.TrimSpace(r.FormValue("payment_method"))

	if err1 != nil || err2 != nil || txID <= 0 || vendor == "" || origAmount <= 0 {
		http.Redirect(w, r, "/transactions?error=Invalid+transaction+update+details", http.StatusSeeOther)
		return
	}
	if currency == "" {
		currency = "INR"
	}
	if txDate == "" {
		txDate = time.Now().Format("2006-01-02")
	}
	if method == "" {
		method = "UPI"
	}

	baseAmount, rate, _ := h.CurrencyService.Convert(origAmount, currency, "INR")

	var catID *int64
	if catIDStr != "" {
		if cid, err := strconv.ParseInt(catIDStr, 10, 64); err == nil && cid > 0 {
			var verifiedID int64
			if err := h.DB.QueryRow(`SELECT id FROM categories WHERE id = ? AND user_id = ?`, cid, userID).Scan(&verifiedID); err == nil {
				catID = &verifiedID
			}
		}
	}

	_, err := h.DB.Exec(`
		UPDATE transactions
		SET category_id = ?, amount = ?, original_amount = ?, original_currency = ?, exchange_rate = ?,
		    vendor = ?, description = ?, date = ?, payment_method = ?
		WHERE id = ? AND user_id = ?`,
		catID, baseAmount, origAmount, currency, rate, vendor, desc, txDate, method, txID, userID,
	)
	if err != nil {
		http.Redirect(w, r, "/transactions?error=Failed+to+update+transaction", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/transactions?success=Transaction+updated", http.StatusSeeOther)
}

func (h *TransactionHandler) ResolveAnomaly(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	txID := r.FormValue("id")
	_, _ = h.DB.Exec(`UPDATE transactions SET is_anomaly = 0, anomaly_reason = '' WHERE id = ? AND user_id = ?`, txID, userID)
	http.Redirect(w, r, "/transactions?success=Anomaly+marked+safe", http.StatusSeeOther)
}

func (h *TransactionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	txID := r.FormValue("id")

	_, _ = h.DB.Exec(`DELETE FROM transactions WHERE id = ? AND user_id = ?`, txID, userID)
	http.Redirect(w, r, "/transactions?success=Transaction+deleted", http.StatusSeeOther)
}

func (h *TransactionHandler) ImportCSV(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		http.Redirect(w, r, "/transactions?error=Failed+to+read+uploaded+CSV+file", http.StatusSeeOther)
		return
	}
	file, _, err := r.FormFile("csvfile")
	if err != nil {
		http.Redirect(w, r, "/transactions?error=Please+select+a+CSV+file+to+import", http.StatusSeeOther)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// Build category map for user
	catMap := make(map[string]int64)
	if rows, err := h.DB.Query(`SELECT id, name FROM categories WHERE user_id = ?`, userID); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var name string
			if err := rows.Scan(&id, &name); err == nil {
				catMap[strings.ToLower(strings.TrimSpace(name))] = id
			}
		}
	}

	sysSettings := h.DB.GetSystemSettings()
	imported := 0
	rowIdx := 0

	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(rec) < 3 {
			continue
		}
		rowIdx++
		// Skip header row if first cell looks like "Date"
		if rowIdx == 1 && strings.EqualFold(strings.TrimSpace(rec[0]), "date") {
			continue
		}

		txDate := strings.TrimSpace(rec[0])
		if txDate == "" {
			txDate = time.Now().Format("2006-01-02")
		}
		vendor := strings.TrimSpace(rec[1])
		amtStr := strings.ReplaceAll(strings.TrimSpace(rec[2]), ",", "")
		origAmt, err := strconv.ParseFloat(amtStr, 64)
		if vendor == "" || err != nil || origAmt <= 0 {
			continue
		}

		curr := "INR"
		if len(rec) > 3 && strings.TrimSpace(rec[3]) != "" {
			curr = strings.ToUpper(strings.TrimSpace(rec[3]))
		}
		catName := ""
		if len(rec) > 4 {
			catName = strings.TrimSpace(rec[4])
		}
		desc := "Bulk CSV Import"
		if len(rec) > 5 && strings.TrimSpace(rec[5]) != "" {
			desc = strings.TrimSpace(rec[5])
		}
		method := "Bank Import"
		if len(rec) > 6 && strings.TrimSpace(rec[6]) != "" {
			method = strings.TrimSpace(rec[6])
		}

		var catID *int64
		if cid, ok := catMap[strings.ToLower(catName)]; ok {
			cidCopy := cid
			catID = &cidCopy
		}

		baseAmt, rate, _ := h.CurrencyService.Convert(origAmt, curr, "INR")
		isAnom := 0
		anomReason := ""
		if baseAmt > sysSettings.AnomalyThreshold {
			isAnom = 1
			anomReason = fmt.Sprintf("CSV Import High Charge (₹%.2f)", baseAmt)
		}

		if _, err := h.DB.Exec(`
			INSERT INTO transactions (
				user_id, category_id, amount, original_amount, original_currency, exchange_rate,
				vendor, description, date, source, payment_method, is_recurring, is_anomaly, anomaly_reason
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'csv_import', ?, 0, ?, ?)`,
			userID, catID, baseAmt, origAmt, curr, rate, vendor, desc, txDate, method, isAnom, anomReason,
		); err == nil {
			imported++
		}
	}

	if imported == 0 {
		http.Redirect(w, r, "/transactions?error=No+valid+rows+found.+Format:+date,vendor,amount,currency,category,description", http.StatusSeeOther)
		return
	}

	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'csv_import', ?)`, userID, fmt.Sprintf("Imported %d transactions via CSV", imported))
	http.Redirect(w, r, fmt.Sprintf("/transactions?success=Imported+%d+transactions+from+CSV", imported), http.StatusSeeOther)
}

func (h *TransactionHandler) ExportTemplateCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=pulsematrix_import_template.csv")
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"Date", "Vendor", "Amount", "Currency", "Category", "Description", "Payment Method"})
	today := time.Now().Format("2006-01-02")
	_ = writer.Write([]string{today, "Starbucks Reserve", "12.50", "USD", "Food & Dining", "Morning Espresso", "Credit Card"})
	_ = writer.Write([]string{today, "Swiggy Instamart", "650.00", "INR", "Food & Dining", "Groceries", "UPI"})
	writer.Flush()
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
	_ = writer.Write([]string{"Date", "Vendor", "Amount (INR)", "Original Amount", "Currency", "Payment Method", "Category", "Source", "Description"})

	for rows.Next() {
		var date, vendor, curr, method, cat, source, desc string
		var amt, origAmt float64
		if err := rows.Scan(&date, &vendor, &amt, &origAmt, &curr, &method, &cat, &source, &desc); err == nil {
			_ = writer.Write([]string{date, vendor, fmt.Sprintf("%.2f", amt), fmt.Sprintf("%.2f", origAmt), curr, method, cat, source, desc})
		}
	}

	writer.Flush()
}

// RatesJSON serves live FX rates for the interactive Multi-Currency Converter widget.
func (h *TransactionHandler) RatesJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rates := h.CurrencyService.GetRates()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"base":  "USD",
		"rates": rates,
	})
}
