package scheduler

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"OpenCodeGoPool/internal/cpa"
	"OpenCodeGoPool/internal/model"
	"OpenCodeGoPool/internal/scraper"
	"OpenCodeGoPool/internal/store"
)

type Scheduler struct {
	store    store.Store
	scraper  *scraper.Client
	cpa      *cpa.Syncer
	interval time.Duration
	stop     chan struct{}
	mu       sync.Mutex
	running  bool
}

func New(s store.Store, sc *scraper.Client, cpaSyncer *cpa.Syncer, interval time.Duration) *Scheduler {
	return &Scheduler{
		store:    s,
		scraper:  sc,
		cpa:      cpaSyncer,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		slog.Info("scheduler started", "interval", s.interval)
		s.ScrapeAll(context.Background())

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.ScrapeAll(context.Background())
			case <-s.stop:
				slog.Info("scheduler stopped")
				return
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.stop)
		s.running = false
	}
}

func (s *Scheduler) ScrapeAll(ctx context.Context) {
	accounts, err := s.store.ListActiveAccounts(ctx)
	if err != nil {
		slog.Error("scheduler: list accounts", "error", err)
		return
	}

	slog.Info("scheduler: scraping accounts", "count", len(accounts))

	for _, acc := range accounts {
		s.scrapeOne(ctx, acc)
	}
}

func (s *Scheduler) ScrapeOne(ctx context.Context, accountID string) error {
	acc, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if acc == nil {
		return nil
	}
	s.scrapeOne(ctx, *acc)
	return nil
}

func (s *Scheduler) scrapeOne(ctx context.Context, acc model.Account) {
	if acc.Type == "ollama" {
		s.scrapeOllamaOne(ctx, acc)
		return
	}
	quota, err := s.scraper.FetchQuota(acc.Cookie, acc.WorkspaceID)
	if err != nil {
		slog.Error("scrape quota failed", "account", acc.ID, "error", err)
		s.store.UpdateAccountStatus(ctx, acc.ID, "error", err.Error())
		return
	}

	quota.AccountID = acc.ID
	if err := s.store.SaveQuotaSnapshot(ctx, quota); err != nil {
		slog.Error("save quota snapshot", "account", acc.ID, "error", err)
	}

	if quota.RollingPercent >= 95 {
		s.store.UpdateAccountStatus(ctx, acc.ID, "rate_limited", "Rolling usage at "+strconv.Itoa(quota.RollingPercent)+"%")
	} else {
		s.store.UpdateAccountStatus(ctx, acc.ID, "active", "")
	}

	// Check per-account limits; auto-remove from CPA if exceeded.
	exceeded := false
	if acc.LimitRolling != nil && quota.RollingPercent >= *acc.LimitRolling {
		exceeded = true
	}
	if acc.LimitWeekly != nil && quota.WeeklyPercent >= *acc.LimitWeekly {
		exceeded = true
	}
	if acc.LimitMonthly != nil && quota.MonthlyPercent >= *acc.LimitMonthly {
		exceeded = true
	}
	if exceeded != acc.LimitExceeded {
		if err := s.store.UpdateAccountLimitExceeded(ctx, acc.ID, exceeded); err != nil {
			slog.Error("update limit_exceeded", "account", acc.ID, "error", err)
		} else {
			go s.cpa.Sync(ctx)
		}
	}

	usages, err := s.scraper.FetchUsage(acc.Cookie, acc.WorkspaceID)
	if err != nil {
		slog.Warn("scrape usage failed", "account", acc.ID, "error", err)
		s.store.UpdateAccountStatusMsg(ctx, acc.ID, "usage: "+err.Error())
		return
	}

	if err := s.store.UpsertUsageRecords(ctx, acc.ID, usages); err != nil {
		slog.Error("save usage records", "account", acc.ID, "error", err)
	}

	s.fetchHistory(ctx, acc)

	slog.Info("scrape complete", "account", acc.ID,
		"rolling", quota.RollingPercent,
		"weekly", quota.WeeklyPercent,
		"monthly", quota.MonthlyPercent,
		"limitExceeded", exceeded,
		"usageRecords", len(usages))
}

func (s *Scheduler) fetchHistory(ctx context.Context, acc model.Account) {
	serverFnID, _ := s.store.GetSetting(ctx, "usage_history_server_fn_id")
	if serverFnID == "" {
		var err error
		serverFnID, err = s.scraper.ExtractServerFnID(acc.Cookie, acc.WorkspaceID)
		if err != nil {
			slog.Warn("extract server-fn-id failed", "account", acc.ID, "error", err)
			return
		}
		_ = s.store.SetSetting(ctx, "usage_history_server_fn_id", serverFnID)
	}

	now := time.Now()
	bootstrapKey := "history_bootstrapped_" + acc.ID
	bootstrapped, _ := s.store.GetSetting(ctx, bootstrapKey)

	var months []time.Time
	if bootstrapped == "" {
		for i := 0; i < 3; i++ {
			months = append(months, now.AddDate(0, -i, 0))
		}
	} else {
		months = append(months, now)
		if now.Day() <= 3 {
			months = append(months, now.AddDate(0, -1, 0))
		}
	}

	allOK := true
	for _, t := range months {
		// API expects 0-indexed month (0=January)
		records, err := s.scraper.FetchUsageHistoryMonth(acc.Cookie, acc.WorkspaceID, serverFnID, t.Year(), int(t.Month())-1)
		if err != nil {
			slog.Warn("fetch usage history failed", "account", acc.ID, "year", t.Year(), "month", t.Month(), "error", err)
			if strings.Contains(err.Error(), "HTTP 5") {
				_ = s.store.SetSetting(ctx, "usage_history_server_fn_id", "")
			}
			allOK = false
			continue
		}
		for i := range records {
			records[i].AccountID = acc.ID
		}
		if err := s.store.UpsertUsageDailyHistory(ctx, records); err != nil {
			slog.Error("upsert usage daily history", "account", acc.ID, "error", err)
			allOK = false
		}
	}

	if bootstrapped == "" && allOK {
		_ = s.store.SetSetting(ctx, bootstrapKey, "1")
	}
}

// scrapeOllamaOne polls the Ollama usage endpoint for one account. Unlike
// OpenCode accounts, Ollama keys are reachable with the API key alone (no
// cookie or workspace ID needed) and expose only a monthly quota window.
func (s *Scheduler) scrapeOllamaOne(ctx context.Context, acc model.Account) {
	u, err := s.scraper.FetchOllamaUsage(acc.APIKey)
	if err != nil {
		slog.Error("scrape ollama usage failed", "account", acc.ID, "error", err)
		s.store.UpdateAccountStatus(ctx, acc.ID, "error", err.Error())
		return
	}

	u.AccountID = acc.ID
	if err := s.store.SaveOllamaUsage(ctx, u); err != nil {
		slog.Error("save ollama usage", "account", acc.ID, "error", err)
	}

	// Ollama exposes a single monthly quota; rolling/weekly are not reported.
	exceeded := false
	if acc.LimitMonthly != nil && u.MonthlyPercent >= *acc.LimitMonthly {
		exceeded = true
	}
	if exceeded != acc.LimitExceeded {
		if err := s.store.UpdateAccountLimitExceeded(ctx, acc.ID, exceeded); err != nil {
			slog.Error("update limit_exceeded", "account", acc.ID, "error", err)
		} else {
			go s.cpa.Sync(ctx)
		}
	}
	if err := s.store.UpdateAccountStatus(ctx, acc.ID, "active", ""); err != nil {
		slog.Error("update status", "account", acc.ID, "error", err)
	}

	slog.Info("ollama scrape complete", "account", acc.ID,
		"monthly", u.MonthlyPercent,
		"limitExceeded", exceeded,
		"models", len(u.Models))
}
