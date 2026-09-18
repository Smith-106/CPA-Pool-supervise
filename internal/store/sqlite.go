package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"OpenCodeGoPool/internal/model"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLite(dbPath string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- Accounts ---

const accountColumns = `id, type, email, cookie, workspace_id, api_key, status, status_msg, limit_rolling, limit_weekly, limit_monthly, limit_exceeded, created_at, updated_at`

func (s *SQLiteStore) ListAccounts(ctx context.Context) ([]model.Account, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+accountColumns+` FROM accounts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccounts(rows)
}

func (s *SQLiteStore) ListActiveAccounts(ctx context.Context) ([]model.Account, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+accountColumns+` FROM accounts WHERE status IN ('active', 'rate_limited') AND limit_exceeded = 0 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccounts(rows)
}

func (s *SQLiteStore) GetAccount(ctx context.Context, id string) (*model.Account, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+accountColumns+` FROM accounts WHERE id = ?`, id)
	acc := &model.Account{}
	err := row.Scan(&acc.ID, &acc.Type, &acc.Email, &acc.Cookie, &acc.WorkspaceID, &acc.APIKey, &acc.Status, &acc.StatusMsg,
		&acc.LimitRolling, &acc.LimitWeekly, &acc.LimitMonthly, &acc.LimitExceeded, &acc.CreatedAt, &acc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return acc, err
}

func (s *SQLiteStore) CreateAccount(ctx context.Context, acc *model.Account) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO accounts (id, type, email, cookie, workspace_id, api_key, status, status_msg, limit_rolling, limit_weekly, limit_monthly, limit_exceeded, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		acc.ID, acc.Type, acc.Email, acc.Cookie, acc.WorkspaceID, acc.APIKey, acc.Status, acc.StatusMsg,
		acc.LimitRolling, acc.LimitWeekly, acc.LimitMonthly, acc.LimitExceeded, acc.CreatedAt, acc.UpdatedAt)
	return err
}

func (s *SQLiteStore) UpdateAccount(ctx context.Context, acc *model.Account) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE accounts SET type = ?, email = ?, cookie = ?, workspace_id = ?, api_key = ?, status = ?, status_msg = ?, limit_rolling = ?, limit_weekly = ?, limit_monthly = ?, updated_at = ? WHERE id = ?`,
		acc.Type, acc.Email, acc.Cookie, acc.WorkspaceID, acc.APIKey, acc.Status, acc.StatusMsg,
		acc.LimitRolling, acc.LimitWeekly, acc.LimitMonthly, time.Now().UTC(), acc.ID)
	return err
}

func (s *SQLiteStore) DeleteAccount(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) UpdateAccountStatus(ctx context.Context, id, status, msg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE accounts SET status = ?, status_msg = ?, updated_at = ? WHERE id = ?`,
		status, msg, time.Now().UTC(), id)
	return err
}

func (s *SQLiteStore) UpdateAccountStatusMsg(ctx context.Context, id, msg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE accounts SET status_msg = ?, updated_at = ? WHERE id = ?`,
		msg, time.Now().UTC(), id)
	return err
}

func (s *SQLiteStore) UpdateAccountLimitExceeded(ctx context.Context, id string, exceeded bool) error {
	val := 0
	if exceeded {
		val = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE accounts SET limit_exceeded = ?, updated_at = ? WHERE id = ?`,
		val, time.Now().UTC(), id)
	return err
}

// --- Quota ---

func (s *SQLiteStore) SaveQuotaSnapshot(ctx context.Context, snap *model.QuotaSnapshot) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO quota_snapshots (id, account_id, rolling_percent, rolling_status, rolling_reset_sec, weekly_percent, weekly_status, weekly_reset_sec, monthly_percent, monthly_status, monthly_reset_sec, scraped_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.ID, snap.AccountID, snap.RollingPercent, snap.RollingStatus, snap.RollingResetSec,
		snap.WeeklyPercent, snap.WeeklyStatus, snap.WeeklyResetSec,
		snap.MonthlyPercent, snap.MonthlyStatus, snap.MonthlyResetSec, snap.ScrapedAt)
	return err
}

func (s *SQLiteStore) GetLatestQuota(ctx context.Context, accountID string) (*model.QuotaSnapshot, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, account_id, rolling_percent, rolling_status, rolling_reset_sec, weekly_percent, weekly_status, weekly_reset_sec, monthly_percent, monthly_status, monthly_reset_sec, scraped_at FROM quota_snapshots WHERE account_id = ? ORDER BY scraped_at DESC LIMIT 1`, accountID)
	snap := &model.QuotaSnapshot{}
	err := row.Scan(&snap.ID, &snap.AccountID, &snap.RollingPercent, &snap.RollingStatus, &snap.RollingResetSec,
		&snap.WeeklyPercent, &snap.WeeklyStatus, &snap.WeeklyResetSec,
		&snap.MonthlyPercent, &snap.MonthlyStatus, &snap.MonthlyResetSec, &snap.ScrapedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return snap, err
}

func (s *SQLiteStore) ListQuotaHistory(ctx context.Context, accountID string, limit int) ([]model.QuotaSnapshot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, account_id, rolling_percent, rolling_status, rolling_reset_sec, weekly_percent, weekly_status, weekly_reset_sec, monthly_percent, monthly_status, monthly_reset_sec, scraped_at FROM quota_snapshots WHERE account_id = ? ORDER BY scraped_at DESC LIMIT ?`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []model.QuotaSnapshot
	for rows.Next() {
		var snap model.QuotaSnapshot
		if err := rows.Scan(&snap.ID, &snap.AccountID, &snap.RollingPercent, &snap.RollingStatus, &snap.RollingResetSec,
			&snap.WeeklyPercent, &snap.WeeklyStatus, &snap.WeeklyResetSec,
			&snap.MonthlyPercent, &snap.MonthlyStatus, &snap.MonthlyResetSec, &snap.ScrapedAt); err != nil {
			return nil, err
		}
		snaps = append(snaps, snap)
	}
	return snaps, rows.Err()
}

// --- Usage ---

func (s *SQLiteStore) UpsertUsageRecords(ctx context.Context, accountID string, records []model.UsageRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO usage_records (id, account_id, model, provider, input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cost, time_created, plan, scraped_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, r := range records {
		_, err := stmt.ExecContext(ctx, r.ID, accountID, r.Model, r.Provider,
			r.InputTokens, r.OutputTokens, r.ReasoningTokens, r.CacheReadTokens,
			r.Cost, r.TimeCreated, r.Plan, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) ListUsageRecords(ctx context.Context, accountID string, limit, offset int) ([]model.UsageRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, account_id, model, provider, input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cost, time_created, plan, scraped_at FROM usage_records WHERE account_id = ? ORDER BY time_created DESC LIMIT ? OFFSET ?`,
		accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.UsageRecord
	for rows.Next() {
		var r model.UsageRecord
		if err := rows.Scan(&r.ID, &r.AccountID, &r.Model, &r.Provider,
			&r.InputTokens, &r.OutputTokens, &r.ReasoningTokens, &r.CacheReadTokens,
			&r.Cost, &r.TimeCreated, &r.Plan, &r.ScrapedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *SQLiteStore) UpsertUsageDailyHistory(ctx context.Context, records []model.UsageDailyHistory) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO usage_daily_history (account_id, date, model, key_id, plan, total_cost, scraped_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, r := range records {
		if _, err := stmt.ExecContext(ctx, r.AccountID, r.Date, r.Model, r.KeyID, r.Plan, r.TotalCost, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) ListUsageDailySummary(ctx context.Context, accountID string) ([]model.UsageDailySummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT date, model, '' AS provider, 0, 0, 0, 0, SUM(total_cost) AS cost
		FROM usage_daily_history
		WHERE account_id = ?
		GROUP BY date, model

		UNION ALL

		SELECT substr(time_created, 1, 10) AS date, model, provider,
		       SUM(input_tokens), SUM(output_tokens),
		       COALESCE(SUM(reasoning_tokens), 0),
		       SUM(cache_read_tokens), SUM(cost) AS cost
		FROM usage_records
		WHERE account_id = ?
		  AND length(time_created) >= 10
		  AND substr(time_created, 1, 4) != '0001'
		  AND substr(time_created, 1, 10) NOT IN (
		      SELECT DISTINCT date FROM usage_daily_history WHERE account_id = ?
		  )
		GROUP BY substr(time_created, 1, 10), model, provider

		ORDER BY date DESC, cost DESC
	`, accountID, accountID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []model.UsageDailySummary
	for rows.Next() {
		var row model.UsageDailySummary
		if err := rows.Scan(&row.Date, &row.Model, &row.Provider,
			&row.InputTokens, &row.OutputTokens, &row.ReasoningTokens,
			&row.CacheReadTokens, &row.Cost); err != nil {
			return nil, err
		}
		summaries = append(summaries, row)
	}
	return summaries, rows.Err()
}

// --- CPA Sync Log ---

func (s *SQLiteStore) SaveSyncLog(ctx context.Context, log *model.CPASyncLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cpa_sync_log (id, status, message, key_count, synced_at) VALUES (?, ?, ?, ?, ?)`,
		log.ID, log.Status, log.Message, log.KeyCount, log.SyncedAt)
	return err
}

func (s *SQLiteStore) GetLatestSyncLog(ctx context.Context) (*model.CPASyncLog, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, status, message, key_count, synced_at FROM cpa_sync_log ORDER BY synced_at DESC LIMIT 1`)
	log := &model.CPASyncLog{}
	err := row.Scan(&log.ID, &log.Status, &log.Message, &log.KeyCount, &log.SyncedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return log, err
}

// --- Settings ---

func (s *SQLiteStore) GetSetting(ctx context.Context, key string) (string, error) {
	var val string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (s *SQLiteStore) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value)
	return err
}

func (s *SQLiteStore) GetCPASettings(ctx context.Context) (*model.CPASettings, error) {
	settings := &model.CPASettings{}

	settings.Endpoint, _ = s.GetSetting(ctx, "cpa_endpoint")
	settings.BearerToken, _ = s.GetSetting(ctx, "cpa_bearer_token")
	settings.ProviderName, _ = s.GetSetting(ctx, "cpa_provider_name")
	settings.BaseURL, _ = s.GetSetting(ctx, "cpa_base_url")

	modelsJSON, _ := s.GetSetting(ctx, "cpa_models")
	if modelsJSON != "" {
		json.Unmarshal([]byte(modelsJSON), &settings.Models)
	}
	if len(settings.Models) == 0 {
		settings.Models = model.DefaultCPAModels
	}

	return settings, nil
}

func (s *SQLiteStore) SaveCPASettings(ctx context.Context, settings *model.CPASettings) error {
	modelsJSON, err := json.Marshal(settings.Models)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, kv := range []struct{ k, v string }{
		{"cpa_endpoint", settings.Endpoint},
		{"cpa_bearer_token", settings.BearerToken},
		{"cpa_provider_name", settings.ProviderName},
		{"cpa_base_url", settings.BaseURL},
		{"cpa_models", string(modelsJSON)},
	} {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			kv.k, kv.v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Dashboard ---

func (s *SQLiteStore) ListAccountsWithQuota(ctx context.Context) ([]model.AccountWithQuota, error) {
	accounts, err := s.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]model.AccountWithQuota, len(accounts))
	for i, acc := range accounts {
		result[i] = model.AccountWithQuota{Account: acc}
		if acc.Type == "ollama" {
			// Ollama accounts report monthly usage from /api/usage, not the
			// OpenCode dashboard quota snapshots.
			usage, err := s.GetLatestOllamaUsage(ctx, acc.ID)
			if err == nil {
				result[i].OllamaUsage = usage
			}
			continue
		}
		quota, err := s.GetLatestQuota(ctx, acc.ID)
		if err != nil {
			continue
		}
		result[i].Quota = quota
	}
	return result, nil
}

// --- helpers ---

func scanAccounts(rows *sql.Rows) ([]model.Account, error) {
	var accounts []model.Account
	for rows.Next() {
		var acc model.Account
		if err := rows.Scan(&acc.ID, &acc.Type, &acc.Email, &acc.Cookie, &acc.WorkspaceID, &acc.APIKey, &acc.Status, &acc.StatusMsg,
			&acc.LimitRolling, &acc.LimitWeekly, &acc.LimitMonthly, &acc.LimitExceeded, &acc.CreatedAt, &acc.UpdatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}

// --- Ollama usage ---

func (s *SQLiteStore) SaveOllamaUsage(ctx context.Context, u *model.OllamaUsage) error {
	models, _ := json.Marshal(u.Models)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO ollama_usage (account_id, monthly_percent, models_json, scraped_at) VALUES (?, ?, ?, ?)`,
		u.AccountID, u.MonthlyPercent, string(models), u.ScrapedAt)
	return err
}

func (s *SQLiteStore) GetLatestOllamaUsage(ctx context.Context, accountID string) (*model.OllamaUsage, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT account_id, monthly_percent, models_json, scraped_at FROM ollama_usage WHERE account_id = ? ORDER BY scraped_at DESC LIMIT 1`, accountID)
	u := &model.OllamaUsage{}
	var modelsJSON string
	err := row.Scan(&u.AccountID, &u.MonthlyPercent, &modelsJSON, &u.ScrapedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.ID = u.AccountID + "-" + u.ScrapedAt.Format("20060102150405")
	_ = json.Unmarshal([]byte(modelsJSON), &u.Models)
	return u, nil
}
