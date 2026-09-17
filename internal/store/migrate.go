package store

import (
	"database/sql"
	"strings"
)

const schema = `
CREATE TABLE IF NOT EXISTS accounts (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL DEFAULT 'opencode',
    email TEXT NOT NULL,
    cookie TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    api_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    status_msg TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS quota_snapshots (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    rolling_percent INTEGER NOT NULL DEFAULT 0,
    rolling_status TEXT NOT NULL DEFAULT 'ok',
    rolling_reset_sec INTEGER NOT NULL DEFAULT 0,
    weekly_percent INTEGER NOT NULL DEFAULT 0,
    weekly_status TEXT NOT NULL DEFAULT 'ok',
    weekly_reset_sec INTEGER NOT NULL DEFAULT 0,
    monthly_percent INTEGER NOT NULL DEFAULT 0,
    monthly_status TEXT NOT NULL DEFAULT 'ok',
    monthly_reset_sec INTEGER NOT NULL DEFAULT 0,
    scraped_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS usage_records (
    id TEXT NOT NULL,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cost INTEGER NOT NULL DEFAULT 0,
    time_created DATETIME NOT NULL,
    plan TEXT NOT NULL DEFAULT '',
    scraped_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, account_id)
);

CREATE TABLE IF NOT EXISTS cpa_sync_log (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    key_count INTEGER NOT NULL DEFAULT 0,
    synced_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS usage_daily_history (
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    date TEXT NOT NULL,
    model TEXT NOT NULL,
    key_id TEXT NOT NULL DEFAULT '',
    plan TEXT NOT NULL DEFAULT '',
    total_cost INTEGER NOT NULL DEFAULT 0,
    scraped_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (account_id, date, model, key_id)
);

CREATE INDEX IF NOT EXISTS idx_quota_account ON quota_snapshots(account_id, scraped_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_account ON usage_records(account_id, time_created DESC);
CREATE TABLE IF NOT EXISTS ollama_usage (
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    monthly_percent INTEGER NOT NULL DEFAULT 0,
    models_json TEXT NOT NULL DEFAULT '[]',
    scraped_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_daily_history_account ON usage_daily_history(account_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_ollama_usage_account ON ollama_usage(account_id, scraped_at DESC);
`

func migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	// Add columns introduced after initial schema — ignore "duplicate column" errors.
	for _, stmt := range []string{
		`ALTER TABLE accounts ADD COLUMN limit_rolling INTEGER`,
		`ALTER TABLE accounts ADD COLUMN limit_weekly INTEGER`,
		`ALTER TABLE accounts ADD COLUMN limit_monthly INTEGER`,
		`ALTER TABLE accounts ADD COLUMN limit_exceeded INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE accounts ADD COLUMN type TEXT NOT NULL DEFAULT 'opencode'`,
	} {
		if _, err := db.Exec(stmt); err != nil && !isDuplicateColumn(err) {
			return err
		}
	}
	return nil
}

func isDuplicateColumn(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists")
}
