package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
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

func formatWithCommas(val float64, symbol string, decimals int) string {
	neg := val < 0
	if neg {
		val = -val
	}
	formatStr := fmt.Sprintf("%%.%df", decimals)
	raw := fmt.Sprintf(formatStr, val)

	intPart := raw
	fracPart := ""
	if idx := strings.IndexByte(raw, '.'); idx >= 0 {
		intPart = raw[:idx]
		fracPart = raw[idx:]
	}

	var grouped string
	n := len(intPart)
	if symbol == "₹" && n > 3 {
		last3 := intPart[n-3:]
		rem := intPart[:n-3]
		var chunks []string
		for len(rem) > 2 {
			chunks = append([]string{rem[len(rem)-2:]}, chunks...)
			rem = rem[:len(rem)-2]
		}
		if len(rem) > 0 {
			chunks = append([]string{rem}, chunks...)
		}
		grouped = strings.Join(chunks, ",") + "," + last3
	} else if n > 3 {
		var chunks []string
		rem := intPart
		for len(rem) > 3 {
			chunks = append([]string{rem[len(rem)-3:]}, chunks...)
			rem = rem[:len(rem)-3]
		}
		if len(rem) > 0 {
			chunks = append([]string{rem}, chunks...)
		}
		grouped = strings.Join(chunks, ",")
	} else {
		grouped = intPart
	}

	if neg {
		return "-" + symbol + grouped + fracPart
	}
	return symbol + grouped + fracPart
}

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Database initialization error: %v", err)
	}
	defer database.Close()

	// Ensure single exclusive Owner Admin account exists
	if err := database.EnsureOwnerAccount(cfg.AdminUsername, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Printf("Warning: owner admin bootstrap check: %v", err)
	}

	// Core Services & Background Worker Pool
	metricsService := services.NewMetricsService()
	aiService := services.NewAIService(cfg)
	currencyService := services.NewCurrencyService(cfg)
	intelService := services.NewIntelligenceService(database)
	workerService := services.NewWorkerService(database, currencyService, metricsService)

	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	workerService.Start(workerCtx)

	funcMap := template.FuncMap{
		"dispMoney": func(amount, rate float64, symbol string) string {
			if rate <= 0 {
				rate = 1.0
			}
			if symbol == "" {
				symbol = "₹"
			}
			return formatWithCommas(amount*rate, symbol, 2)
		},
		"dispWhole": func(amount, rate float64, symbol string) string {
			if rate <= 0 {
				rate = 1.0
			}
			if symbol == "" {
				symbol = "₹"
			}
			return formatWithCommas(math.Round(amount*rate), symbol, 0)
		},
		"dispMoneySigned": func(amount, rate float64, symbol string) string {
			if rate <= 0 {
				rate = 1.0
			}
			if symbol == "" {
				symbol = "₹"
			}
			v := amount * rate
			if v < 0 {
				return formatWithCommas(v, symbol, 2)
			}
			return "+" + formatWithCommas(v, symbol, 2)
		},
		"formatMoney": func(f float64) string {
			if f < 0 {
				return formatWithCommas(f, "₹", 2)
			}
			return "+" + formatWithCommas(f, "₹", 2)
		},
		"formatMoneyPlain": func(f float64) string {
			return formatWithCommas(f, "₹", 2)
		},
		"absVal": func(f float64) float64 {
			if f < 0 {
				return -f
			}
			return f
		},
		"formatDate": func(s string) string {
			raw := strings.TrimSpace(s)
			if raw == "" {
				return "—"
			}
			layouts := []string{
				time.RFC3339,
				time.RFC3339Nano,
				"2006-01-02T15:04:05Z",
				"2006-01-02 15:04:05",
				"2006-01-02",
			}
			for _, layout := range layouts {
				if t, err := time.Parse(layout, raw); err == nil {
					return t.Format("02 Jan 2006")
				}
			}
			if len(raw) >= 10 {
				if t, err := time.Parse("2006-01-02", raw[:10]); err == nil {
					return t.Format("02 Jan 2006")
				}
			}
			return raw
		},
		"formatDateTime": func(s string) string {
			raw := strings.TrimSpace(s)
			if raw == "" {
				return "—"
			}
			layouts := []string{
				time.RFC3339,
				time.RFC3339Nano,
				"2006-01-02T15:04:05Z",
				"2006-01-02 15:04:05",
				"2006-01-02",
			}
			for _, layout := range layouts {
				if t, err := time.Parse(layout, raw); err == nil {
					return t.Format("02 Jan 2006, 15:04")
				}
			}
			return raw
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
				c = "#10b981"
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
				c = "#10b981"
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

	loginTmpl := template.Must(template.New("login.html").Funcs(funcMap).ParseFiles("templates/login.html"))
	adminLoginTmpl := template.Must(template.New("admin_login.html").Funcs(funcMap).ParseFiles("templates/admin_login.html"))
	regTmpl := template.Must(template.New("register.html").Funcs(funcMap).ParseFiles("templates/register.html"))

	dashTmpl := page("dashboard.html")
	txnTmpl := page("transactions.html")
	goalsTmpl := page("goals.html")
	splitsTmpl := page("splits.html")
	netWorthTmpl := page("networth.html")
	reportTmpl := page("report.html")
	analyticsTmpl := page("analytics.html")
	subTmpl := page("subscriptions.html")
	aiAdvisorTmpl := page("ai_advisor.html")
	bankSyncTmpl := page("bank_sync.html")
	adminTmpl := page("admin.html")

	authH := &handlers.AuthHandler{
		DB:                 database,
		LoginTemplate:      loginTmpl,
		AdminLoginTemplate: adminLoginTmpl,
		RegisterTemplate:   regTmpl,
		JWTSecret:          cfg.JWTSecret,
		OwnerEmail:         cfg.AdminEmail,
		OwnerAccessKey:     cfg.AdminAccessKey,
		AdminPortalPath:    cfg.AdminPortalPath,
	}
	dashH := handlers.NewDashboardHandler(database, dashTmpl, intelService, currencyService)
	txnH := &handlers.TransactionHandler{DB: database, Templates: txnTmpl, CurrencyService: currencyService}
	goalsH := handlers.NewGoalsHandler(database, goalsTmpl, currencyService)
	splitsH := handlers.NewSplitsHandler(database, splitsTmpl, currencyService)
	netWorthH := handlers.NewNetWorthHandler(database, netWorthTmpl, currencyService)
	reportH := handlers.NewReportHandler(database, reportTmpl, intelService, currencyService)
	analyticsH := &handlers.AnalyticsHandler{DB: database, Templates: analyticsTmpl, IntelService: intelService, CurrencyService: currencyService}
	subH := &handlers.SubscriptionsHandler{DB: database, Templates: subTmpl, CurrencyService: currencyService}
	aiH := handlers.NewAIHandler(database, aiService, intelService, currencyService, aiAdvisorTmpl)
	bankH := handlers.NewBankSyncHandler(database, bankSyncTmpl, aiService, currencyService, metricsService, cfg.JWTSecret, cfg.WebhookSecret)
	adminH := handlers.NewAdminHandler(database, adminTmpl, currencyService, metricsService, workerService)

	r := mux.NewRouter()

	// Production Middleware Chain: Request-ID Tracing + Security Headers + Panic Recovery + Token-Bucket Rate Limiting
	globalLimiter := middleware.NewRateLimiter(240, 60)
	r.Use(middleware.SecurityAndObservability(metricsService))
	r.Use(globalLimiter.Middleware)

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := database.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "pulsematrix-ai", "version": "7.0", "baseCurrency": "INR"})
	}).Methods("GET")

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public User Auth Routes (Zero admin links exposed!)
	r.HandleFunc("/login", authH.ShowLogin).Methods("GET")
	r.HandleFunc("/login", authH.Login).Methods("POST")
	r.HandleFunc("/register", authH.ShowRegister).Methods("GET")
	r.HandleFunc("/register", authH.Register).Methods("POST")
	r.HandleFunc("/logout", authH.Logout).Methods("GET", "POST")

	// Private Owner-Only Stealth Login Gateway (configurable via ADMIN_PORTAL_PATH)
	r.HandleFunc(cfg.AdminPortalPath, authH.ShowAdminLogin).Methods("GET")
	r.HandleFunc(cfg.AdminPortalPath, authH.AdminLogin).Methods("POST")

	// Payment App & SMS Integration Endpoints (Idempotent + HMAC-SHA256 verified)
	r.HandleFunc("/api/webhooks/bank", bankH.WebhookAPI).Methods("POST")
	r.HandleFunc("/api/webhooks/payment", bankH.PaymentAppWebhookAPI).Methods("POST")
	r.HandleFunc("/api/v1/ingest/sms", bankH.SMSIngestionAPI).Methods("POST")

	// Stealth Owner-Only Admin Control Plane (returns 404 Not Found for all non-owners!)
	adminRouter := r.PathPrefix("/admin").Subrouter()
	adminRouter.Use(middleware.Auth(cfg.JWTSecret, database))
	adminRouter.Use(middleware.RequireOwnerAdmin(cfg.AdminEmail))

	adminRouter.HandleFunc("", adminH.Show).Methods("GET")
	adminRouter.HandleFunc("/", adminH.Show).Methods("GET")
	adminRouter.HandleFunc("/metrics", adminH.MetricsJSON).Methods("GET")
	adminRouter.HandleFunc("/metrics/prometheus", adminH.PrometheusMetrics).Methods("GET")
	adminRouter.HandleFunc("/worker/trigger", adminH.TriggerWorker).Methods("POST")
	adminRouter.HandleFunc("/users/create", adminH.CreateUser).Methods("POST")
	adminRouter.HandleFunc("/users/toggle-status", adminH.ToggleStatus).Methods("POST")
	adminRouter.HandleFunc("/users/delete", adminH.DeleteUser).Methods("POST")
	adminRouter.HandleFunc("/settings", adminH.UpdatePlatformSettings).Methods("POST")

	// Protected User Routes
	authed := r.PathPrefix("").Subrouter()
	authed.Use(middleware.Auth(cfg.JWTSecret, database))

	authed.HandleFunc("/", dashH.Show).Methods("GET")

	authed.HandleFunc("/transactions", txnH.List).Methods("GET")
	authed.HandleFunc("/transactions/add", txnH.Add).Methods("POST")
	authed.HandleFunc("/transactions/edit", txnH.Edit).Methods("POST")
	authed.HandleFunc("/transactions/delete", txnH.Delete).Methods("POST")
	authed.HandleFunc("/transactions/resolve-anomaly", txnH.ResolveAnomaly).Methods("POST")
	authed.HandleFunc("/transactions/import/csv", txnH.ImportCSV).Methods("POST")
	authed.HandleFunc("/transactions/export/csv", txnH.ExportCSV).Methods("GET")
	authed.HandleFunc("/transactions/template/csv", txnH.ExportTemplateCSV).Methods("GET")

	authed.HandleFunc("/goals", goalsH.Show).Methods("GET")
	authed.HandleFunc("/goals/add", goalsH.AddGoal).Methods("POST")
	authed.HandleFunc("/goals/contribute", goalsH.ContributeGoal).Methods("POST")
	authed.HandleFunc("/goals/delete", goalsH.DeleteGoal).Methods("POST")
	authed.HandleFunc("/budgets/update", goalsH.UpdateCategoryBudget).Methods("POST")
	authed.HandleFunc("/categories/add", goalsH.AddCategory).Methods("POST")

	// Split-Bill & Shared Expenses
	authed.HandleFunc("/splits", splitsH.Show).Methods("GET")
	authed.HandleFunc("/splits/add", splitsH.Add).Methods("POST")
	authed.HandleFunc("/splits/settle", splitsH.Settle).Methods("POST")
	authed.HandleFunc("/splits/delete", splitsH.Delete).Methods("POST")

	// Net Worth (Assets & Liabilities)
	authed.HandleFunc("/networth", netWorthH.Show).Methods("GET")
	authed.HandleFunc("/networth/add", netWorthH.Add).Methods("POST")
	authed.HandleFunc("/networth/update", netWorthH.UpdateValue).Methods("POST")
	authed.HandleFunc("/networth/delete", netWorthH.Delete).Methods("POST")

	// Printable Monthly Executive Financial Statement
	authed.HandleFunc("/report", reportH.Show).Methods("GET")

	authed.HandleFunc("/analytics", analyticsH.Show).Methods("GET")
	authed.HandleFunc("/subscriptions", subH.Show).Methods("GET")
	authed.HandleFunc("/subscriptions/add", subH.Add).Methods("POST")
	authed.HandleFunc("/subscriptions/cancel", subH.Cancel).Methods("POST")

	authed.HandleFunc("/ai-advisor", aiH.ShowAdvisor).Methods("GET")
	authed.HandleFunc("/bank-sync", bankH.Show).Methods("GET")
	authed.HandleFunc("/settings/update", authH.UpdateSettings).Methods("POST")
	authed.HandleFunc("/settings/currency", authH.SwitchCurrency).Methods("POST")

	authed.HandleFunc("/api/ai/parse-receipt", aiH.ParseReceiptAPI).Methods("POST")
	authed.HandleFunc("/api/ai/advisor", aiH.AdvisorChatAPI).Methods("POST")
	authed.HandleFunc("/api/reports/monthly", analyticsH.TrendsJSON).Methods("GET")
	authed.HandleFunc("/api/reports/categories", analyticsH.CategoriesJSON).Methods("GET")
	authed.HandleFunc("/api/currency/rates", txnH.RatesJSON).Methods("GET")

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server & handle SIGINT/SIGTERM Graceful Shutdown
	go func() {
		log.Printf("🚀 PulseMatrix AI v5.0 running at http://localhost:%s (base=%s)", cfg.Port, cfg.BaseCurrency)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("🛑 Received signal %v — shutting down gracefully...", sig)
	cancelWorkers()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown error: %v", err)
	}
	log.Println("✅ PulseMatrix AI exited cleanly")
}
