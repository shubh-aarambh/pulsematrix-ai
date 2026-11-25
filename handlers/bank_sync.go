package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
	"expense-matrix/services"
)

type BankSyncHandler struct {
	DB              *db.DB
	Templates       *template.Template
	AIService       *services.AIService
	CurrencyService *services.CurrencyService
	JWTSecret       string
}

func NewBankSyncHandler(database *db.DB, tmpl *template.Template, ai *services.AIService, cs *services.CurrencyService, jwtSecret string) *BankSyncHandler {
	return &BankSyncHandler{
		DB:              database,
		Templates:       tmpl,
		AIService:       ai,
		CurrencyService: cs,
		JWTSecret:       jwtSecret,
	}
}

func (h *BankSyncHandler) resolveWebhookUserID(r *http.Request) (int64, error) {
	if uid, ok := r.Context().Value(middleware.UserIDKey).(int64); ok && uid > 0 {
		return uid, nil
	}
	if uid, ok := ExtractOptionalUserID(r, h.JWTSecret); ok && uid > 0 {
		return uid, nil
	}
	var firstUserID int64
	if err := h.DB.QueryRow(`SELECT id FROM users ORDER BY id ASC LIMIT 1`).Scan(&firstUserID); err != nil {
		return 0, fmt.Errorf("no registered user found to receive webhook")
	}
	return firstUserID, nil
}

func (h *BankSyncHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, userID, username)

	rows, err := h.DB.Query(`
		SELECT id, action, details, created_at
		FROM audit_logs
		WHERE user_id = ? AND (action LIKE 'bank_%' OR action LIKE 'payment_%' OR action LIKE 'sms_%')
		ORDER BY created_at DESC, id DESC LIMIT 15`, userID)

	type LogView struct {
		ID        int64
		Action    string
		Details   string
		CreatedAt string
	}

	var logs []LogView
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var l LogView
			if err := rows.Scan(&l.ID, &l.Action, &l.Details, &l.CreatedAt); err == nil {
				logs = append(logs, l)
			}
		}
	}

	viewCtx["Title"] = "Payment Apps & Open Webhooks - PulseMatrix AI"
	viewCtx["ActivePage"] = "bank-sync"
	viewCtx["Logs"] = logs

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}

// WebhookAPI handles Plaid & traditional bank webhook feeds.
func (h *BankSyncHandler) WebhookAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var payload models.BankWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "Invalid webhook JSON body")
		return
	}

	payload.Merchant = strings.TrimSpace(payload.Merchant)
	if payload.Merchant == "" || payload.Amount <= 0 {
		WriteJSONError(w, http.StatusBadRequest, "Missing required fields (merchant, amount)")
		return
	}

	userID, err := h.resolveWebhookUserID(r)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if payload.Currency == "" {
		payload.Currency = "USD"
	} else {
		payload.Currency = strings.ToUpper(strings.TrimSpace(payload.Currency))
	}

	txDate := strings.TrimSpace(payload.Date)
	if txDate == "" {
		txDate = time.Now().Format("2006-01-02")
	}

	baseAmount, rate, _ := h.CurrencyService.Convert(payload.Amount, payload.Currency, "USD")

	var catID *int64
	if payload.Category != "" {
		var cid int64
		if err := h.DB.QueryRow(`SELECT id FROM categories WHERE user_id = ? AND name = ?`, userID, payload.Category).Scan(&cid); err == nil && cid > 0 {
			catID = &cid
		}
	}

	var existingCount int
	_ = h.DB.QueryRow(`
		SELECT COUNT(*) FROM transactions
		WHERE user_id = ? AND vendor = ? AND original_amount = ? AND date = ?`,
		userID, payload.Merchant, payload.Amount, txDate).Scan(&existingCount)

	isAnomaly := 0
	anomalyReason := ""
	if existingCount > 0 {
		isAnomaly = 1
		anomalyReason = "Automated Bank Webhook Duplicate Charge Warning"
	}

	_, err = h.DB.Exec(`
		INSERT INTO transactions (
			user_id, category_id, amount, original_amount, original_currency, exchange_rate,
			vendor, description, date, source, payment_method, is_recurring, is_anomaly, anomaly_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'Plaid Automated Bank Ingestion', ?, 'bank_sync', 'Bank Webhook', 0, ?, ?)`,
		userID, catID, baseAmount, payload.Amount, payload.Currency, rate, payload.Merchant, txDate, isAnomaly, anomalyReason,
	)

	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to record transaction: %v", err))
		return
	}

	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'bank_webhook_received', ?)`,
		userID, fmt.Sprintf("Ingested %.2f %s ($%.2f USD) from %s (TxID: %s)", payload.Amount, payload.Currency, baseAmount, payload.Merchant, payload.TransactionID),
	)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "success",
		"message":          "Bank transaction ingested into ledger",
		"transactionId":    payload.TransactionID,
		"normalizedAmount": baseAmount,
		"isAnomaly":        isAnomaly == 1,
	})
}

// SMSIngestionAPI handles SMS transaction notifications forwarded from Google Pay / PhonePe / Paytm / Bank SMS.
func (h *BankSyncHandler) SMSIngestionAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		SMS    string `json:"sms"`
		Sender string `json:"sender"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.SMS) == "" {
		WriteJSONError(w, http.StatusBadRequest, "SMS text body is required")
		return
	}

	userID, err := h.resolveWebhookUserID(r)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	parsed, err := h.AIService.ParseReceiptOrText(req.SMS)
	if err != nil || parsed == nil {
		WriteJSONError(w, http.StatusInternalServerError, "Failed to parse SMS transaction text")
		return
	}

	if parsed.Amount <= 0 {
		WriteJSONError(w, http.StatusBadRequest, "Could not detect a valid transaction amount in SMS text")
		return
	}

	baseAmount, rate, _ := h.CurrencyService.Convert(parsed.Amount, parsed.Currency, "USD")

	var catID *int64
	if parsed.SuggestedCategory != "" {
		var cid int64
		if err := h.DB.QueryRow(`SELECT id FROM categories WHERE user_id = ? AND name = ?`, userID, parsed.SuggestedCategory).Scan(&cid); err == nil && cid > 0 {
			catID = &cid
		}
	}

	isAnomaly := 0
	if parsed.IsAnomaly {
		isAnomaly = 1
	}

	method := "Mobile Payment App"
	if strings.TrimSpace(req.Sender) != "" {
		method = strings.TrimSpace(req.Sender)
	}

	_, err = h.DB.Exec(`
		INSERT INTO transactions (
			user_id, category_id, amount, original_amount, original_currency, exchange_rate,
			vendor, description, date, source, payment_method, is_recurring, is_anomaly, anomaly_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'sms_auto_sync', ?, 0, ?, ?)`,
		userID, catID, baseAmount, parsed.Amount, parsed.Currency, rate,
		parsed.Vendor, "Automated Payment App SMS Ingestion", parsed.Date, method, isAnomaly, parsed.AnomalyReason,
	)

	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to store transaction: %v", err))
		return
	}

	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'sms_payment_synced', ?)`,
		userID, fmt.Sprintf("Parsed Payment SMS from %s: %.2f %s ($%.2f USD) at %s", method, parsed.Amount, parsed.Currency, baseAmount, parsed.Vendor),
	)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "success",
		"message":          "Payment SMS parsed and added to ledger",
		"extractedData":    parsed,
		"normalizedAmount": baseAmount,
		"paymentAppSender": method,
	})
}

// PaymentAppWebhookAPI handles direct webhooks from Google Pay, PhonePe, Paytm, Stripe, Razorpay, PayPal.
func (h *BankSyncHandler) PaymentAppWebhookAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var payload struct {
		AppName       string  `json:"appName"`
		TxnID         string  `json:"txnId"`
		Merchant      string  `json:"merchant"`
		Amount        float64 `json:"amount"`
		Currency      string  `json:"currency"`
		Category      string  `json:"category"`
		PaymentMethod string  `json:"paymentMethod"`
		Date          string  `json:"date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Merchant) == "" || payload.Amount <= 0 {
		WriteJSONError(w, http.StatusBadRequest, "Invalid payment webhook payload")
		return
	}

	userID, err := h.resolveWebhookUserID(r)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if payload.Currency == "" {
		payload.Currency = "USD"
	} else {
		payload.Currency = strings.ToUpper(strings.TrimSpace(payload.Currency))
	}
	if payload.Date == "" {
		payload.Date = time.Now().Format("2006-01-02")
	}
	if payload.AppName == "" {
		payload.AppName = "Payment App"
	}
	if payload.PaymentMethod == "" {
		payload.PaymentMethod = "UPI / Wallet"
	}

	baseAmount, rate, _ := h.CurrencyService.Convert(payload.Amount, payload.Currency, "USD")

	var catID *int64
	if payload.Category != "" {
		var cid int64
		_ = h.DB.QueryRow(`SELECT id FROM categories WHERE user_id = ? AND name = ?`, userID, payload.Category).Scan(&cid)
		if cid > 0 {
			catID = &cid
		}
	}

	_, err = h.DB.Exec(`
		INSERT INTO transactions (
			user_id, category_id, amount, original_amount, original_currency, exchange_rate,
			vendor, description, date, source, payment_method, is_recurring, is_anomaly, anomaly_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'payment_app_api', ?, 0, 0, '')`,
		userID, catID, baseAmount, payload.Amount, payload.Currency, rate,
		strings.TrimSpace(payload.Merchant), fmt.Sprintf("Direct %s API Sync (Txn: %s)", payload.AppName, payload.TxnID), payload.Date, payload.PaymentMethod,
	)

	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to log payment transaction: %v", err))
		return
	}

	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'payment_webhook_received', ?)`,
		userID, fmt.Sprintf("Ingested %.2f %s ($%.2f USD) from %s via %s (Txn: %s)", payload.Amount, payload.Currency, baseAmount, payload.Merchant, payload.AppName, payload.TxnID),
	)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "success",
		"message":          fmt.Sprintf("Payment from %s successfully recorded into PulseMatrix Ledger", payload.AppName),
		"txnId":            payload.TxnID,
		"normalizedAmount": baseAmount,
	})
}
