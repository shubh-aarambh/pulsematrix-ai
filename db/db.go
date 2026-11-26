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
    preferred_currency  TEXT NOT NULL DEFAULT 'INR',
    monthly_budget_goal REAL NOT NULL DEFAULT 75000.0,
    last_login_at       TEXT DEFAULT '',
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS categories (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    icon          TEXT NOT NULL DEFAULT '📦',
    color         TEXT NOT NULL DEFAULT '#10b981',
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
    original_currency TEXT NOT NULL DEFAULT 'INR',
    exchange_rate     REAL NOT NULL DEFAULT 1.0,
    vendor            TEXT NOT NULL DEFAULT '',
    description       TEXT NOT NULL DEFAULT '',
    date              DATE NOT NULL,
    source            TEXT NOT NULL DEFAULT 'manual',
    payment_method    TEXT NOT NULL DEFAULT 'UPI',
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
    currency          TEXT NOT NULL DEFAULT 'INR',
    category_name     TEXT NOT NULL DEFAULT 'Subscriptions & SaaS',
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

CREATE TABLE IF NOT EXISTS split_expenses (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    person_name TEXT NOT NULL,
    description TEXT NOT NULL,
    amount      REAL NOT NULL,
    currency    TEXT NOT NULL DEFAULT 'INR',
    direction   TEXT NOT NULL CHECK(direction IN ('owed_to_me','i_owe')),
    status      TEXT NOT NULL DEFAULT 'pending',
    due_date    DATE NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS net_worth_items (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    category   TEXT NOT NULL DEFAULT 'Bank',
    kind       TEXT NOT NULL CHECK(kind IN ('asset','liability')),
    value      REAL NOT NULL DEFAULT 0,
    icon       TEXT NOT NULL DEFAULT '🏦',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
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

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key        TEXT PRIMARY KEY,
    response   TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_transactions_user_date ON transactions(user_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_cat       ON transactions(category_id);
CREATE INDEX IF NOT EXISTS idx_transactions_vendor    ON transactions(user_id, vendor);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user     ON subscriptions(user_id, status);
CREATE INDEX IF NOT EXISTS idx_goals_user             ON savings_goals(user_id, status);
CREATE INDEX IF NOT EXISTS idx_splits_user            ON split_expenses(user_id, status);
CREATE INDEX IF NOT EXISTS idx_networth_user          ON net_worth_items(user_id, kind);
`
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	_ = db.ensureColumn("users", "role", "TEXT NOT NULL DEFAULT 'user'")
	_ = db.ensureColumn("users", "status", "TEXT NOT NULL DEFAULT 'active'")
	_ = db.ensureColumn("users", "last_login_at", "TEXT DEFAULT ''")

	defaults := map[string]string{
		"allow_registration":  "true",
		"announcement_banner": "",
		"anomaly_threshold":   "25000",
		"default_budget_goal": "75000",
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

// EnsureOwnerAccount guarantees the single Owner Admin account exists and matches configured credentials.
func (db *DB) EnsureOwnerAccount(username, email, password string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	var existingID int64
	err = db.QueryRow(`SELECT id FROM users WHERE lower(email) = lower(?) OR lower(username) = lower(?) LIMIT 1`, email, username).Scan(&existingID)
	if err == nil && existingID > 0 {
		_, _ = db.Exec(`UPDATE users SET username = ?, email = ?, password = ?, role = 'admin', status = 'active' WHERE id = ?`, username, email, string(hashed), existingID)
		// Demote any non-owner admin accounts so ONLY the owner has admin privileges
		_, _ = db.Exec(`UPDATE users SET role = 'user' WHERE id != ? AND role = 'admin'`, existingID)
		return nil
	}

	// Demote any legacy admin accounts first
	_, _ = db.Exec(`UPDATE users SET role = 'user' WHERE role = 'admin'`)

	res, err := db.Exec(`
		INSERT INTO users (username, email, password, role, status, preferred_currency, monthly_budget_goal)
		VALUES (?, ?, ?, 'admin', 'active', 'INR', 85000.0)`,
		username, email, string(hashed),
	)
	if err != nil {
		return err
	}
	if adminID, err := res.LastInsertId(); err == nil && adminID > 0 {
		_ = db.SeedDefaults(adminID)
		_, _ = db.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'owner_bootstrap', 'Owner administrator initialized')`, adminID)
	}
	return nil
}

func (db *DB) GetSystemSettings() models.SystemSettings {
	s := models.SystemSettings{
		AllowRegistration:  true,
		AnnouncementBanner: "",
		AnomalyThreshold:   25000.0,
		DefaultBudgetGoal:  75000.0,
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

// SeedDefaults creates INR-native default categories, transactions, subscriptions, goals, splits, and net worth items.
func (db *DB) SeedDefaults(userID int64) error {
	categories := []struct {
		name, icon, color, typ string
		limit                  float64
	}{
		{"Salary & Income", "💰", "#10b981", "income", 0},
		{"Housing & Utilities", "🏠", "#06b6d4", "expense", 35000},
		{"Food & Dining", "🍜", "#f59e0b", "expense", 15000},
		{"Technology & Cloud", "💻", "#14b8a6", "expense", 12000},
		{"Travel & Transport", "✈️", "#0ea5e9", "expense", 10000},
		{"Shopping & Retail", "🛍️", "#fb7185", "expense", 12000},
		{"Subscriptions & SaaS", "🎬", "#eab308", "expense", 6000},
		{"Investments & SIP", "📈", "#22c55e", "investment", 30000},
		{"Miscellaneous", "📦", "#64748b", "expense", 5000},
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

	today := time.Now()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE user_id = ?`, userID).Scan(&count); err == nil && count == 0 {
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
			{"Salary & Income", "Apex FinTech Systems", 185000.00, "Monthly Principal Engineer Payroll NEFT", "bank_sync", "Bank Transfer", 1, false, false, ""},
			{"Housing & Utilities", "Prestige Living Estates", 32000.00, "Monthly Apartment Rent & Maintenance", "bank_sync", "UPI AutoPay", 3, true, false, ""},
			{"Technology & Cloud", "Amazon Web Services", 6450.00, "AWS Production Cluster & S3 Storage", "ai_ocr", "Credit Card", 4, true, false, ""},
			{"Technology & Cloud", "OpenAI / Gemini API", 2400.00, "LLM Developer API Credits", "ai_ocr", "Credit Card", 5, true, false, ""},
			{"Food & Dining", "Third Wave Coffee", 680.00, "Cold Brew & Sourdough Croissant", "payment_app_api", "Google Pay / UPI", 1, false, false, ""},
			{"Food & Dining", "Swiggy Instamart", 3420.00, "Weekly Fresh Groceries & Pantry Stock", "sms_auto_sync", "PhonePe / UPI", 2, false, false, ""},
			{"Food & Dining", "Taj Masala Kraft", 11800.00, "Team Milestone Dinner - High Value Dining", "ai_ocr", "Credit Card", 6, false, true, "Surge Spend: 310% higher than dining average"},
			{"Subscriptions & SaaS", "Netflix Ultra HD 4K", 649.00, "Monthly Streaming Plan", "bank_sync", "UPI AutoPay", 8, true, false, ""},
			{"Subscriptions & SaaS", "GitHub Copilot Pro", 1650.00, "AI Coding Assistant Subscription", "bank_sync", "Credit Card", 10, true, false, ""},
			{"Subscriptions & SaaS", "Spotify Family Duo", 179.00, "Music Streaming Plan", "bank_sync", "UPI AutoPay", 12, true, false, ""},
			{"Travel & Transport", "Uber Premier Airport", 1240.00, "Airport Express Cab", "payment_app_api", "Google Pay / UPI", 7, false, false, ""},
			{"Travel & Transport", "Uber Premier Airport", 1240.00, "Duplicate Billing Detected within 4 mins", "bank_sync", "Google Pay / UPI", 7, false, true, "Potential Duplicate Billing Detected"},
			{"Shopping & Retail", "Apple India Store", 14900.00, "AirPods Pro 2 & MagSafe Charger", "ai_ocr", "Credit Card", 14, false, false, ""},
			{"Investments & SIP", "Zerodha Coin Nifty 50", 25000.00, "Automated Monthly Index Fund SIP", "bank_sync", "UPI AutoPay", 16, true, false, ""},
			{"Salary & Income", "Architecture Advisory", 35000.00, "System Design Consulting Retainer", "manual", "IMPS Transfer", 19, false, false, ""},
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
				) VALUES (?, ?, ?, ?, 'INR', 1.0, ?, ?, ?, ?, ?, ?, ?, ?)`,
				userID, catIDPtr, s.amount, s.amount, s.vendor, s.desc, txDate, s.source, s.method,
				s.rec, s.anom, s.reason,
			)
		}

		subs := []struct {
			name, cycle, cat string
			amt              float64
			daysFromNow      int
		}{
			{"AWS Cloud Infrastructure", "monthly", "Technology & Cloud", 6450.00, 4},
			{"Netflix Ultra HD 4K", "monthly", "Subscriptions & SaaS", 649.00, 9},
			{"GitHub Copilot Pro", "monthly", "Subscriptions & SaaS", 1650.00, 14},
			{"Spotify Family", "monthly", "Subscriptions & SaaS", 179.00, 7},
			{"AI API Developer Tier", "monthly", "Technology & Cloud", 2400.00, 19},
		}

		for _, sub := range subs {
			nextDate := today.AddDate(0, 0, sub.daysFromNow).Format("2006-01-02")
			_, _ = db.Exec(`
				INSERT INTO subscriptions (user_id, name, amount, currency, category_name, billing_cycle, next_billing_date, status, detected_pattern)
				VALUES (?, ?, ?, 'INR', ?, ?, ?, 'active', 'Detected via UPI AutoPay / Bank Sync')`,
				userID, sub.name, sub.amt, sub.cat, sub.cycle, nextDate,
			)
		}
	}

	var goalCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM savings_goals WHERE user_id = ?`, userID).Scan(&goalCount); err == nil && goalCount == 0 {
		goals := []struct {
			name, icon, cat string
			target, current float64
			monthsAhead     int
		}{
			{"6-Month Emergency Reserve", "🛡️", "Safety Net", 500000.0, 345000.0, 8},
			{"Japan & Europe Travel Fund", "✈️", "Travel", 250000.0, 162000.0, 5},
			{"MacBook Pro M4 Workstation", "💻", "Hardware", 220000.0, 155000.0, 3},
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

	var splitCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM split_expenses WHERE user_id = ?`, userID).Scan(&splitCount); err == nil && splitCount == 0 {
		splits := []struct {
			person, desc, dir string
			amt               float64
			daysAhead         int
		}{
			{"Aarav Mehta", "Goa Weekend Villa Share", "owed_to_me", 8500.0, 5},
			{"Priya Sharma", "Team Dinner Split at Taj", "owed_to_me", 2950.0, 3},
			{"Rohan Verma", "Flat Fiber & Electricity Split", "i_owe", 1850.0, 4},
		}
		for _, sp := range splits {
			due := today.AddDate(0, 0, sp.daysAhead).Format("2006-01-02")
			_, _ = db.Exec(`
				INSERT INTO split_expenses (user_id, person_name, description, amount, currency, direction, status, due_date)
				VALUES (?, ?, ?, ?, 'INR', ?, 'pending', ?)`,
				userID, sp.person, sp.desc, sp.amt, sp.dir, due,
			)
		}
	}

	var nwCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM net_worth_items WHERE user_id = ?`, userID).Scan(&nwCount); err == nil && nwCount == 0 {
		items := []struct {
			name, cat, kind, icon string
			val                   float64
		}{
			{"HDFC Salary & Savings Account", "Bank Cash", "asset", "🏦", 420000.0},
			{"Zerodha Nifty 50 & Equity SIP", "Mutual Funds & Stocks", "asset", "📈", 865000.0},
			{"Sovereign Gold Bonds", "Gold", "asset", "🪙", 185000.0},
			{"Emergency Liquid FD", "Fixed Deposit", "asset", "🛡️", 300000.0},
			{"HDFC Infinia Credit Card Due", "Credit Card", "liability", "💳", 28500.0},
			{"Two-Wheeler / Vehicle EMI", "Loan", "liability", "🏍️", 85000.0},
		}
		for _, it := range items {
			_, _ = db.Exec(`
				INSERT INTO net_worth_items (user_id, name, category, kind, value, icon)
				VALUES (?, ?, ?, ?, ?, ?)`,
				userID, it.name, it.cat, it.kind, it.val, it.icon,
			)
		}
	}

	return nil
}
