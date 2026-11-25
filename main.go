package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"expense-matrix/config"
	"expense-matrix/db"
	"expense-matrix/handlers"
	"expense-matrix/middleware"
	"expense-matrix/services"

	"github.com/gorilla/mux"
)

var validHexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Database initialization error: %v", err)
	}
	defer database.Close()

	// Initialize Services
	aiService := services.NewAIService(cfg)
	currencyService := services.NewCurrencyService(cfg)
	intelService := services.NewIntelligenceService(database)

	// Seed defaults only if initial user already exists
	var userOneExists int
	if err := database.QueryRow(`SELECT COUNT(*) FROM users WHERE id = 1`).Scan(&userOneExists); err == nil && userOneExists > 0 {
		_ = database.SeedDefaults(1)
	}

	// Template Helper Functions
	funcMap := template.FuncMap{
		"formatMoney": func(f float64) string {
			if f < 0 {
				return fmt.Sprintf("-$%.2f", -f)
			}
			return fmt.Sprintf("+$%.2f", f)
		},
		"formatMoneyPlain": func(f float64) string {
			return fmt.Sprintf("$%.2f", f)
		},
		"absVal": func(f float64) float64 {
			if f < 0 {
				return -f
			}
			return f
		},
		"formatDate": func(s string) string {
			t, err := time.Parse("2006-01-02", s)
			if err != nil {
				return s
			}
			return t.Format("02 Jan 2006")
		},
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
		"initial": func(s string) string {
			for _, r := range strings.TrimSpace(s) {
				return string(unicode.ToUpper(r))
			}
			return "U"
		},
		"clampPct": func(f float64) float64 {
			if f < 0 {
				return 0
			}
			if f > 100 {
				return 100
			}
			return f
		},
		"safeBorderStyle": func(hex string) template.HTMLAttr {
			c := strings.TrimSpace(hex)
			if !validHexColorRe.MatchString(c) {
				c = "#6366f1"
			}
			return template.HTMLAttr(fmt.Sprintf(`style="border-left: 3px solid %s;"`, c))
		},
		"safeWidthStyle": func(pct float64) template.HTMLAttr {
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
			return template.HTMLAttr(fmt.Sprintf(`style="width: %.1f%%;"`, pct))
		},
		"now": func() string { return time.Now().Format("2006-01-02") },
	}

	// Helper to build page templates with base layout & flash partial
	page := func(filename string) *template.Template {
		return template.Must(
			template.New("base.html").Funcs(funcMap).ParseFiles(
				"templates/base.html",
				"templates/partials/flash.html",
				"templates/"+filename,
			),
		)
	}

	// Auth standalone templates
	loginTmpl := template.Must(template.New("login.html").Funcs(funcMap).ParseFiles("templates/login.html"))
	regTmpl := template.Must(template.New("register.html").Funcs(funcMap).ParseFiles("templates/register.html"))

	// View Templates
	dashTmpl := page("dashboard.html")
	txnTmpl := page("transactions.html")
	analyticsTmpl := page("analytics.html")
	subTmpl := page("subscriptions.html")
	aiAdvisorTmpl := page("ai_advisor.html")
	bankSyncTmpl := page("bank_sync.html")

	// Handlers
	authH := &handlers.AuthHandler{DB: database, LoginTemplate: loginTmpl, RegisterTemplate: regTmpl, JWTSecret: cfg.JWTSecret}
	dashH := handlers.NewDashboardHandler(database, dashTmpl, intelService)
	txnH := &handlers.TransactionHandler{DB: database, Templates: txnTmpl, CurrencyService: currencyService}
	analyticsH := &handlers.AnalyticsHandler{DB: database, Templates: analyticsTmpl, IntelService: intelService}
	subH := &handlers.SubscriptionsHandler{DB: database, Templates: subTmpl}
	aiH := handlers.NewAIHandler(database, aiService, intelService, aiAdvisorTmpl)
	bankH := handlers.NewBankSyncHandler(database, bankSyncTmpl, aiService, currencyService, cfg.JWTSecret)

	// Gorilla Mux Router
	r := mux.NewRouter()

	// Health check endpoint (for cloud platforms & Docker health checks)
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := database.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "pulsematrix-ai"})
	}).Methods("GET")

	// Static Assets Server
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public Auth Routes
	r.HandleFunc("/login", authH.ShowLogin).Methods("GET")
	r.HandleFunc("/login", authH.Login).Methods("POST")
	r.HandleFunc("/register", authH.ShowRegister).Methods("GET")
	r.HandleFunc("/register", authH.Register).Methods("POST")
	r.HandleFunc("/logout", authH.Logout).Methods("GET", "POST")

	// Payment App Integration Endpoints (supports both external webhooks & logged-in browser simulator)
	r.HandleFunc("/api/webhooks/bank", bankH.WebhookAPI).Methods("POST")
	r.HandleFunc("/api/webhooks/payment", bankH.PaymentAppWebhookAPI).Methods("POST")
	r.HandleFunc("/api/v1/ingest/sms", bankH.SMSIngestionAPI).Methods("POST")

	// Protected Routes (JWT Required)
	authed := r.PathPrefix("").Subrouter()
	authed.Use(middleware.Auth(cfg.JWTSecret))

	authed.HandleFunc("/", dashH.Show).Methods("GET")
	authed.HandleFunc("/transactions", txnH.List).Methods("GET")
	authed.HandleFunc("/transactions/add", txnH.Add).Methods("POST")
	authed.HandleFunc("/transactions/delete", txnH.Delete).Methods("POST")
	authed.HandleFunc("/transactions/export/csv", txnH.ExportCSV).Methods("GET")

	authed.HandleFunc("/analytics", analyticsH.Show).Methods("GET")
	authed.HandleFunc("/subscriptions", subH.Show).Methods("GET")
	authed.HandleFunc("/subscriptions/add", subH.Add).Methods("POST")
	authed.HandleFunc("/subscriptions/cancel", subH.Cancel).Methods("POST")

	authed.HandleFunc("/ai-advisor", aiH.ShowAdvisor).Methods("GET")
	authed.HandleFunc("/bank-sync", bankH.Show).Methods("GET")
	authed.HandleFunc("/settings/update", authH.UpdateSettings).Methods("POST")

	// REST API Endpoints
	authed.HandleFunc("/api/ai/parse-receipt", aiH.ParseReceiptAPI).Methods("POST")
	authed.HandleFunc("/api/ai/advisor", aiH.AdvisorChatAPI).Methods("POST")
	authed.HandleFunc("/api/reports/monthly", analyticsH.TrendsJSON).Methods("GET")
	authed.HandleFunc("/api/reports/categories", analyticsH.CategoriesJSON).Methods("GET")

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("🚀 PulseMatrix AI Platform running at http://localhost:%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
