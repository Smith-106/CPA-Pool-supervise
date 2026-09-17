package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"OpenCodeGoPool/internal/model"
)

// opencodeAPIUsageURL is the JSON quota endpoint on the OpenCode Go API.
// It accepts the same API key used for model requests, so no cookie is
// needed — this is the preferred path when the account has an API key.
const opencodeAPIUsageURL = "https://opencode.ai/zen/go/v1/usage"

// opencodeAPIUsage mirrors the JSON shape returned by GET /zen/go/v1/usage.
type opencodeAPIUsage struct {
	RollingUsage struct {
		UsagePercent float64 `json:"usage_percent"`
		ResetInSec   int64   `json:"reset_in_sec"`
	} `json:"rolling_usage"`
	WeeklyUsage struct {
		UsagePercent float64 `json:"usage_percent"`
		ResetInSec   int64   `json:"reset_in_sec"`
	} `json:"weekly_usage"`
	MonthlyUsage struct {
		UsagePercent float64 `json:"usage_percent"`
		ResetInSec   int64   `json:"reset_in_sec"`
	} `json:"monthly_usage"`
}

// FetchQuotaByAPIKey calls the OpenCode Go usage endpoint with the account's
// API key. Returns nil,nil when the key is rejected (caller should fall back
// to the cookie-based dashboard scrape).
func (c *Client) FetchQuotaByAPIKey(apiKey string) (*model.QuotaSnapshot, error) {
	req, err := http.NewRequest(http.MethodGet, opencodeAPIUsageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, nil // key rejected — caller falls back to cookie path
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var raw opencodeAPIUsage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse quota: %w", err)
	}

	return &model.QuotaSnapshot{
		AccountID:      "",
		RollingPercent:  int(raw.RollingUsage.UsagePercent),
		RollingStatus:   "ok",
		RollingResetSec: int(raw.RollingUsage.ResetInSec),
		WeeklyPercent:   int(raw.WeeklyUsage.UsagePercent),
		WeeklyStatus:    "ok",
		WeeklyResetSec:  int(raw.WeeklyUsage.ResetInSec),
		MonthlyPercent:  int(raw.MonthlyUsage.UsagePercent),
		MonthlyStatus:   "ok",
		MonthlyResetSec: int(raw.MonthlyUsage.ResetInSec),
		ScrapedAt:       time.Now().UTC(),
	}, nil
}
