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

	// Guarantee a default Platform Administrator exists on startup
	if err := database.EnsureAdminAccount(cfg.AdminUsername, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Printf("Warning: admin bootstrap check: %v", err)
	}

	// Initialize Services
	aiService := services.NewAIService(cfg)
	currencyService := services.NewCurrencyService(cfg)
	intelService := services.NewIntelligenceService(database)

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
		"derefInt64": func(p *int64) int64 {
			if p == nil {
				return 0
			}
			return *p
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
		"safeBgColor": func(hex string) template.HTMLAttr {
			c := strings.TrimSpace(hex)
			if !validHexColorRe.MatchString(c) {
				c = "#6366f1"
			}
			return template.HTMLAttr(fmt.Sprintf(`style="background:%s;"`, c))
		},
		"now": func() string { return time.Now().Format("2006-01-02") },
	}

	page := func(filename string) *template.Template {
		return template.Must(
			template.New("base.html").Funcs(funcMap).ParseFiles(
				"templates/base.html",
				"templates/partials/flash.html",
				"templates/"+filename,
			),
		)
	}

	// Auth standalone templates (separate User Login, Admin Login, and Register)
	loginTmpl := template.Must(template.New("login.html").Funcs(funcMap).ParseFiles("templates/login.html"))
	adminLoginTmpl := template.Must(template.New("admin_login.html").Funcs(funcMap).ParseFiles("templates/admin_login.html"))
	regTmpl := template.Must(template.New("register.html").Funcs(funcMap).ParseFiles("templates/register.html"))

	// Application View Templates
	dashTmpl := page("dashboard.html")
	txnTmpl := page("transactions.html")
	goalsTmpl := page("goals.html")
	analyticsTmpl := page("analytics.html")
	subTmpl := page("subscriptions.html")
	aiAdvisorTmpl := page("ai_advisor.html")
	bankSyncTmpl := page("bank_sync.html")
	adminTmpl := page("admin.html")

	// Handlers
	authH := &handlers.AuthHandler{
		DB:                 database,
		LoginTemplate:      loginTmpl,
		AdminLoginTemplate: adminLoginTmpl,
		RegisterTemplate:   regTmpl,
		JWTSecret:          cfg.JWTSecret,
	}
	dashH := handlers.NewDashboardHandler(database, dashTmpl, intelService)
	txnH := &handlers.TransactionHandler{DB: database, Templates: txnTmpl, CurrencyService: currencyService}
	goalsH := handlers.NewGoalsHandler(database, goalsTmpl)
	analyticsH := &handlers.AnalyticsHandler{DB: database, Templates: analyticsTmpl, IntelService: intelService}
	subH := &handlers.SubscriptionsHandler{DB: database, Templates: subTmpl}
	aiH := handlers.NewAIHandler(database, aiService, intelService, aiAdvisorTmpl)
	bankH := handlers.NewBankSyncHandler(database, bankSyncTmpl, aiService, currencyService, cfg.JWTSecret)
	adminH := handlers.NewAdminHandler(database, adminTmpl)

	r := mux.NewRouter()

	// Health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := database.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "pulsematrix-ai", "version": "3.0"})
	}).Methods("GET")

	// Static Assets Server
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public Auth Routes (Separate User & Admin Portals)
	r.HandleFunc("/login", authH.ShowLogin).Methods("GET")
	r.HandleFunc("/login", authH.Login).Methods("POST")
	r.HandleFunc("/admin/login", authH.ShowAdminLogin).Methods("GET")
	r.HandleFunc("/admin/login", authH.AdminLogin).Methods("POST")
	r.HandleFunc("/register", authH.ShowRegister).Methods("GET")
	r.HandleFunc("/register", authH.Register).Methods("POST")
	r.HandleFunc("/logout", authH.Logout).Methods("GET", "POST")

	// Payment App & SMS Integration Endpoints
	r.HandleFunc("/api/webhooks/bank", bankH.WebhookAPI).Methods("POST")
	r.HandleFunc("/api/webhooks/payment", bankH.PaymentAppWebhookAPI).Methods("POST")
	r.HandleFunc("/api/v1/ingest/sms", bankH.SMSIngestionAPI).Methods("POST")

	// Protected Admin Portal Routes (JWT + Admin Role Required)
	adminRouter := r.PathPrefix("/admin").Subrouter()
	adminRouter.Use(middleware.Auth(cfg.JWTSecret, database))
	adminRouter.Use(middleware.RequireAdmin())

	adminRouter.HandleFunc("", adminH.Show).Methods("GET")
	adminRouter.HandleFunc("/", adminH.Show).Methods("GET")
	adminRouter.HandleFunc("/users/create", adminH.CreateUser).Methods("POST")
	adminRouter.HandleFunc("/users/toggle-role", adminH.ToggleRole).Methods("POST")
	adminRouter.HandleFunc("/users/toggle-status", adminH.ToggleStatus).Methods("POST")
	adminRouter.HandleFunc("/users/delete", adminH.DeleteUser).Methods("POST")
	adminRouter.HandleFunc("/settings", adminH.UpdatePlatformSettings).Methods("POST")

	// Protected User Routes (JWT Required)
	authed := r.PathPrefix("").Subrouter()
	authed.Use(middleware.Auth(cfg.JWTSecret, database))

	authed.HandleFunc("/", dashH.Show).Methods("GET")

	// Transactions + Edit + Anomaly Resolution + CSV Import/Export
	authed.HandleFunc("/transactions", txnH.List).Methods("GET")
	authed.HandleFunc("/transactions/add", txnH.Add).Methods("POST")
	authed.HandleFunc("/transactions/edit", txnH.Edit).Methods("POST")
	authed.HandleFunc("/transactions/delete", txnH.Delete).Methods("POST")
	authed.HandleFunc("/transactions/resolve-anomaly", txnH.ResolveAnomaly).Methods("POST")
	authed.HandleFunc("/transactions/import/csv", txnH.ImportCSV).Methods("POST")
	authed.HandleFunc("/transactions/export/csv", txnH.ExportCSV).Methods("GET")
	authed.HandleFunc("/transactions/template/csv", txnH.ExportTemplateCSV).Methods("GET")

	// Budgets, Custom Categories & Savings Goals Hub
	authed.HandleFunc("/goals", goalsH.Show).Methods("GET")
	authed.HandleFunc("/goals/add", goalsH.AddGoal).Methods("POST")
	authed.HandleFunc("/goals/contribute", goalsH.ContributeGoal).Methods("POST")
	authed.HandleFunc("/goals/delete", goalsH.DeleteGoal).Methods("POST")
	authed.HandleFunc("/budgets/update", goalsH.UpdateCategoryBudget).Methods("POST")
	authed.HandleFunc("/categories/add", goalsH.AddCategory).Methods("POST")

	// Analytics, Subscriptions, AI Advisor, Integrations, Settings
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
	authed.HandleFunc("/api/currency/rates", txnH.RatesJSON).Methods("GET")

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("🚀 PulseMatrix AI Platform v3.0 running at http://localhost:%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
