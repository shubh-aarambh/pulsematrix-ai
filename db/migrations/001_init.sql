-- 001_init.sql (Reference schema matching db/db.go)
CREATE TABLE IF NOT EXISTS users (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    username            TEXT NOT NULL UNIQUE,
    email               TEXT NOT NULL UNIQUE,
    password            TEXT NOT NULL,
    preferred_currency  TEXT NOT NULL DEFAULT 'USD',
    monthly_budget_goal REAL NOT NULL DEFAULT 4500.0,
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
