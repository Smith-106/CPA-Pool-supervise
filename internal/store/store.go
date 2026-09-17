package store

import (
	"context"

	"OpenCodeGoPool/internal/model"
)

type Store interface {
	// Accounts
	ListAccounts(ctx context.Context) ([]model.Account, error)
	ListActiveAccounts(ctx context.Context) ([]model.Account, error)
	GetAccount(ctx context.Context, id string) (*model.Account, error)
	CreateAccount(ctx context.Context, acc *model.Account) error
	UpdateAccount(ctx context.Context, acc *model.Account) error
	DeleteAccount(ctx context.Context, id string) error
	UpdateAccountStatus(ctx context.Context, id, status, msg string) error
	UpdateAccountStatusMsg(ctx context.Context, id, msg string) error
	UpdateAccountLimitExceeded(ctx context.Context, id string, exceeded bool) error

	// Quota
	SaveQuotaSnapshot(ctx context.Context, snap *model.QuotaSnapshot) error
	GetLatestQuota(ctx context.Context, accountID string) (*model.QuotaSnapshot, error)
	ListQuotaHistory(ctx context.Context, accountID string, limit int) ([]model.QuotaSnapshot, error)

	// Usage
	UpsertUsageRecords(ctx context.Context, accountID string, records []model.UsageRecord) error
	ListUsageRecords(ctx context.Context, accountID string, limit, offset int) ([]model.UsageRecord, error)
	ListUsageDailySummary(ctx context.Context, accountID string) ([]model.UsageDailySummary, error)
	UpsertUsageDailyHistory(ctx context.Context, records []model.UsageDailyHistory) error

	// CPA Sync Log
	SaveSyncLog(ctx context.Context, log *model.CPASyncLog) error
	GetLatestSyncLog(ctx context.Context) (*model.CPASyncLog, error)

	// Settings
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	GetCPASettings(ctx context.Context) (*model.CPASettings, error)
	SaveCPASettings(ctx context.Context, settings *model.CPASettings) error

	// Ollama usage
	SaveOllamaUsage(ctx context.Context, u *model.OllamaUsage) error
	GetLatestOllamaUsage(ctx context.Context, accountID string) (*model.OllamaUsage, error)

	// Dashboard
	ListAccountsWithQuota(ctx context.Context) ([]model.AccountWithQuota, error)

	Close() error
}
