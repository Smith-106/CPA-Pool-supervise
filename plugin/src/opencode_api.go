package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

// opencodeAPIUsageURL is the JSON quota endpoint on the OpenCode Go API.
// It accepts the same API key used for model requests, so no cookie is
// needed — this is the preferred path when the account has an API key.
const opencodeAPIUsageURL = "https://opencode.ai/zen/go/v1/usage"

// opencodeAPIUsage mirrors the JSON shape returned by GET /zen/go/v1/usage.
// Field names are inferred from the OpenCode API surface; unknown fields are
// ignored so the monitor keeps working when the schema grows.
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

// fetchOpenCodeUsageByAPIKey calls the OpenCode Go usage endpoint with the
// account's API key and returns the parsed snapshot. Returns nil,nil when the
// endpoint is not reachable with this key (caller should fall back to the
// cookie-based dashboard scrape).
func fetchOpenCodeUsageByAPIKey(apiKey string) (*windowUsageSet, error) {
	resp, err := hostHTTPDo(pluginapi.HTTPRequest{
		Method: http.MethodGet,
		URL:    opencodeAPIUsageURL,
		Headers: http.Header{
			"Authorization": []string{"Bearer " + apiKey},
			"Accept":        []string{"application/json"},
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		// API key rejected — the caller should try the cookie path instead.
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode usage HTTP %d", resp.StatusCode)
	}
	var raw opencodeAPIUsage
	if errUnmarshal := json.Unmarshal(resp.Body, &raw); errUnmarshal != nil {
		return nil, fmt.Errorf("opencode usage parse: %w", errUnmarshal)
	}
	return &windowUsageSet{
		FiveHour: windowUsage{UsagePercent: pctInt(raw.RollingUsage.UsagePercent), ResetInSec: raw.RollingUsage.ResetInSec},
		Weekly:   windowUsage{UsagePercent: pctInt(raw.WeeklyUsage.UsagePercent), ResetInSec: raw.WeeklyUsage.ResetInSec},
		Monthly:  windowUsage{UsagePercent: pctInt(raw.MonthlyUsage.UsagePercent), ResetInSec: raw.MonthlyUsage.ResetInSec},
	}, nil
}

// windowUsageSet groups the three quota windows for a single account.
type windowUsageSet struct {
	FiveHour windowUsage
	Weekly   windowUsage
	Monthly  windowUsage
}
