package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"expense-matrix/models"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	sqldb, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db := &DB{sqldb}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    username            TEXT NOT NULL UNIQUE,
    email               TEXT NOT NULL UNIQUE,
    password            TEXT NOT NULL,
    role                TEXT NOT NULL DEFAULT 'user',
    status              TEXT NOT NULL DEFAULT 'active',
    preferred_currency  TEXT NOT NULL DEFAULT 'USD',
    monthly_budget_goal REAL NOT NULL DEFAULT 4500.0,
    last_login_at       TEXT DEFAULT '',
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS categories (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    icon          TEXT NOT NULL DEFAULT '📦',
    color         TEXT NOT NULL DEFAULT '#6366f1',
    type          TEXT NOT NULL CHECK(type IN ('expense','income','investment')),
    monthly_limit REAL DEFAULT 0,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, name)
);

CREATE TABLE IF NOT EXISTS transactions (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id       INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    amount            REAL NOT NULL,
    original_amount   REAL NOT NULL,
    original_currency TEXT NOT NULL DEFAULT 'USD',
    exchange_rate     REAL NOT NULL DEFAULT 1.0,
    vendor            TEXT NOT NULL DEFAULT '',
    description       TEXT NOT NULL DEFAULT '',
    date              DATE NOT NULL,
    source            TEXT NOT NULL DEFAULT 'manual',
    payment_method    TEXT NOT NULL DEFAULT 'Credit Card',
    is_recurring      INTEGER NOT NULL DEFAULT 0,
    is_anomaly        INTEGER NOT NULL DEFAULT 0,
    anomaly_reason    TEXT DEFAULT '',
    receipt_url       TEXT DEFAULT '',
    notes             TEXT DEFAULT '',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    amount            REAL NOT NULL,
    currency          TEXT NOT NULL DEFAULT 'USD',
    category_name     TEXT NOT NULL DEFAULT 'Subscriptions',
    billing_cycle     TEXT NOT NULL DEFAULT 'monthly',
    next_billing_date DATE NOT NULL,
    status            TEXT NOT NULL DEFAULT 'active',
    detected_pattern  TEXT DEFAULT '',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS savings_goals (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    icon           TEXT NOT NULL DEFAULT '🎯',
    target_amount  REAL NOT NULL,
    current_amount REAL NOT NULL DEFAULT 0,
    target_date    DATE NOT NULL,
    category       TEXT NOT NULL DEFAULT 'Savings',
    status         TEXT NOT NULL DEFAULT 'active',
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS system_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER REFERENCES users(id) ON DELETE CASCADE,
    action     TEXT NOT NULL,
    details    TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_transactions_user_date ON transactions(user_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_cat       ON transactions(category_id);
CREATE INDEX IF NOT EXISTS idx_transactions_vendor    ON transactions(user_id, vendor);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user     ON subscriptions(user_id, status);
CREATE INDEX IF NOT EXISTS idx_goals_user             ON savings_goals(user_id, status);
`
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// Safe incremental migrations for existing databases
	_ = db.ensureColumn("users", "role", "TEXT NOT NULL DEFAULT 'user'")
	_ = db.ensureColumn("users", "status", "TEXT NOT NULL DEFAULT 'active'")
	_ = db.ensureColumn("users", "last_login_at", "TEXT DEFAULT ''")

	// Seed default platform settings
	defaults := map[string]string{
		"allow_registration":  "true",
		"announcement_banner": "",
		"anomaly_threshold":   "800",
		"default_budget_goal": "4500",
	}
	for k, v := range defaults {
		_, _ = db.Exec(`INSERT OR IGNORE INTO system_settings (key, value) VALUES (?, ?)`, k, v)
	}

	return nil
}

func (db *DB) ensureColumn(table, column, colDef string) error {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			if strings.EqualFold(name, column) {
				return nil
			}
		}
	}
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, colDef))
	return err
}

// EnsureAdminAccount guarantees a default administrator exists on fresh installs.
func (db *DB) EnsureAdminAccount(username, email, password string) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count); err == nil && count > 0 {
		return nil
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res, err := db.Exec(`
		INSERT OR IGNORE INTO users (username, email, password, role, status, preferred_currency, monthly_budget_goal)
		VALUES (?, ?, ?, 'admin', 'active', 'USD', 6000.0)`,
		username, email, string(hashed),
	)
	if err != nil {
		return err
	}
	if adminID, err := res.LastInsertId(); err == nil && adminID > 0 {
		_ = db.SeedDefaults(adminID)
		_, _ = db.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'system_bootstrap', 'Default platform administrator initialized')`, adminID)
	}
	return nil
}

// GetSystemSettings retrieves global platform settings configured by admins.
func (db *DB) GetSystemSettings() models.SystemSettings {
	s := models.SystemSettings{
		AllowRegistration:  true,
		AnnouncementBanner: "",
		AnomalyThreshold:   800.0,
		DefaultBudgetGoal:  4500.0,
	}
	rows, err := db.Query(`SELECT key, value FROM system_settings`)
	if err != nil {
		return s
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			switch k {
			case "allow_registration":
				s.AllowRegistration = v == "true"
			case "announcement_banner":
				s.AnnouncementBanner = strings.TrimSpace(v)
			case "anomaly_threshold":
				if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
					s.AnomalyThreshold = f
				}
			case "default_budget_goal":
				if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
					s.DefaultBudgetGoal = f
				}
			}
		}
	}
	return s
}

func (db *DB) SetSystemSetting(key, value string) error {
	_, err := db.Exec(`
		INSERT INTO system_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value,
	)
	return err
}

// SeedDefaults creates default categories, sample transactions, subscriptions, and savings goals for a user.
func (db *DB) SeedDefaults(userID int64) error {
	categories := []struct {
		name, icon, color, typ string
		limit                  float64
	}{
		{"Salary & Income", "💰", "#10b981", "income", 0},
		{"Housing & Utilities", "🏠", "#3b82f6", "expense", 1500},
		{"Food & Dining", "🍜", "#f59e0b", "expense", 600},
		{"Technology & Cloud", "💻", "#6366f1", "expense", 400},
		{"Travel & Transport", "✈️", "#06b6d4", "expense", 500},
		{"Shopping & Retail", "🛍️", "#ec4899", "expense", 300},
		{"Subscriptions & SaaS", "🎬", "#8b5cf6", "expense", 150},
		{"Investments & Crypto", "📈", "#84cc16", "investment", 1000},
		{"Miscellaneous", "📦", "#64748b", "expense", 200},
	}

	for _, c := range categories {
		_, err := db.Exec(
			`INSERT OR IGNORE INTO categories(user_id, name, icon, color, type, monthly_limit) VALUES(?,?,?,?,?,?)`,
			userID, c.name, c.icon, c.color, c.typ, c.limit,
		)
		if err != nil {
			return err
		}
	}

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE user_id = ?`, userID).Scan(&count)
	if err == nil && count == 0 {
		today := time.Now()

		samples := []struct {
			catName string
			vendor  string
			amount  float64
			desc    string
			source  string
			method  string
			daysAgo int
			rec     bool
			anom    bool
			reason  string
		}{
			{"Salary & Income", "Global Tech Corp", 6800.00, "Monthly Tech Lead Payroll Direct Deposit", "bank_sync", "Direct Deposit", 1, false, false, ""},
			{"Housing & Utilities", "Apex Living Management", 1450.00, "Monthly Apartment Rent Payment", "bank_sync", "ACH Bank Transfer", 3, true, false, ""},
			{"Technology & Cloud", "Amazon Web Services", 284.50, "AWS Cloud Architecture Infrastructure Usage", "ai_ocr", "Credit Card", 4, true, false, ""},
			{"Technology & Cloud", "OpenAI / API Billing", 60.00, "LLM Enterprise API Credits", "ai_ocr", "Credit Card", 5, true, false, ""},
			{"Food & Dining", "Sweetgreen Organic", 18.75, "Lunch Salad & Smoothie Bowl", "manual", "Apple Pay", 2, false, false, ""},
			{"Food & Dining", "Whole Foods Market", 142.30, "Weekly Grocery & Gourmet Stock", "ai_ocr", "Credit Card", 6, false, false, ""},
			{"Food & Dining", "Noma Fine Dining", 480.00, "Team Milestone Dinner - High Value Charge", "ai_ocr", "Credit Card", 7, false, true, "Surge Spend: 340% higher than category mean"},
			{"Subscriptions & SaaS", "Netflix Premium 4K", 22.99, "Monthly Streaming Service Subscription", "bank_sync", "Credit Card", 10, true, false, ""},
			{"Subscriptions & SaaS", "GitHub Copilot Enterprise", 19.00, "AI Developer Assistant Subscription", "bank_sync", "Credit Card", 12, true, false, ""},
			{"Subscriptions & SaaS", "Spotify Premium", 10.99, "Audio Streaming Subscription", "bank_sync", "Credit Card", 14, true, false, ""},
			{"Travel & Transport", "Uber Technologies", 34.50, "Airport Express Ride", "manual", "Apple Pay", 8, false, false, ""},
			{"Travel & Transport", "Uber Technologies", 34.50, "Duplicate Charge Detected within 5 mins", "bank_sync", "Apple Pay", 8, false, true, "Potential Duplicate Billing Detected"},
			{"Shopping & Retail", "Apple Store", 1299.00, "MacBook M3 Pro Workstation Upgrade", "ai_ocr", "Credit Card", 15, false, false, ""},
			{"Investments & Crypto", "Vanguard Index Funds", 850.00, "Monthly Automated S&P 500 DCA Investment", "bank_sync", "ACH Bank Transfer", 18, true, false, ""},
			{"Salary & Income", "Consulting Retainer", 1500.00, "AI Architecture Advisory Retainer", "manual", "Wire Transfer", 22, false, false, ""},
		}

		for _, s := range samples {
			var catID int64
			_ = db.QueryRow(`SELECT id FROM categories WHERE user_id = ? AND name = ?`, userID, s.catName).Scan(&catID)
			txDate := today.AddDate(0, 0, -s.daysAgo).Format("2006-01-02")

			var catIDPtr *int64
			if catID > 0 {
				catIDPtr = &catID
			}

			_, _ = db.Exec(`
				INSERT INTO transactions (
					user_id, category_id, amount, original_amount, original_currency, exchange_rate,
					vendor, description, date, source, payment_method, is_recurring, is_anomaly, anomaly_reason
				) VALUES (?, ?, ?, ?, 'USD', 1.0, ?, ?, ?, ?, ?, ?, ?, ?)`,
				userID, catIDPtr, s.amount, s.amount, s.vendor, s.desc, txDate, s.source, s.method,
				s.rec, s.anom, s.reason,
			)
		}

		subs := []struct {
			name, cycle, cat string
			amt              float64
			daysFromNow      int
		}{
			{"Netflix Premium", "monthly", "Subscriptions & SaaS", 22.99, 12},
			{"AWS Cloud Services", "monthly", "Technology & Cloud", 284.50, 5},
			{"GitHub Copilot", "monthly", "Subscriptions & SaaS", 19.00, 18},
			{"Spotify Family", "monthly", "Subscriptions & SaaS", 16.99, 8},
			{"OpenAI Enterprise", "monthly", "Technology & Cloud", 60.00, 20},
		}

		for _, sub := range subs {
			nextDate := today.AddDate(0, 0, sub.daysFromNow).Format("2006-01-02")
			_, _ = db.Exec(`
				INSERT INTO subscriptions (user_id, name, amount, currency, category_name, billing_cycle, next_billing_date, status, detected_pattern)
				VALUES (?, ?, ?, 'USD', ?, ?, ?, 'active', 'Detected via recurring bank webhook')`,
				userID, sub.name, sub.amt, sub.cat, sub.cycle, nextDate,
			)
		}
	}

	var goalCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM savings_goals WHERE user_id = ?`, userID).Scan(&goalCount); err == nil && goalCount == 0 {
		today := time.Now()
		goals := []struct {
			name, icon, cat string
			target, current float64
			monthsAhead     int
		}{
			{"6-Month Emergency Reserve", "🛡️", "Safety Net", 15000.0, 9400.0, 8},
			{"Japan & Europe Travel Fund", "✈️", "Travel", 4500.0, 2850.0, 5},
			{"Next-Gen AI Workstation", "💻", "Hardware", 3200.0, 2100.0, 3},
		}
		for _, g := range goals {
			tDate := today.AddDate(0, g.monthsAhead, 0).Format("2006-01-02")
			_, _ = db.Exec(`
				INSERT INTO savings_goals (user_id, name, icon, target_amount, current_amount, target_date, category, status)
				VALUES (?, ?, ?, ?, ?, ?, ?, 'active')`,
				userID, g.name, g.icon, g.target, g.current, tDate, g.cat,
			)
		}
	}

	return nil
}
