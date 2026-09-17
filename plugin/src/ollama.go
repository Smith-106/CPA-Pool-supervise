package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

// ollamaUsageURL is the account usage endpoint exposed by ollama.com. It is
// the same endpoint consumed by Kosello/ollama-cloud-watch; unlike OpenCode
// dashboard polling it is reachable with the API key alone (Bearer), so no
// cookie is required.
const ollamaUsageURL = "https://ollama.com/api/usage"

// ollamaUsageResponse mirrors the JSON shape returned by GET /api/usage.
// Only the fields needed for quota protection are decoded; unknown fields
// are ignored so the monitor keeps working when Ollama adds data.
type ollamaUsageResponse struct {
	Limits struct {
		Monthly struct {
			Usage  float64 `json:"usage"`
			Models []struct {
				Name         string `json:"name"`
				RequestCount int    `json:"request_count"`
			} `json:"models"`
		} `json:"monthly"`
		Weekly *struct {
			Usage float64 `json:"usage"`
		} `json:"weekly"`
		Session *struct {
			Usage float64 `json:"usage"`
		} `json:"session"`
	} `json:"limits"`
}

// ollamaUsage is the parsed quota snapshot for one Ollama account.
type ollamaUsage struct {
	MonthlyPercent int
	WeeklyPercent  int
	SessionPercent int
	Models         []ollamaModelUsage
	FetchedAt      time.Time
}

type ollamaModelUsage struct {
	Name         string `json:"name"`
	RequestCount int    `json:"request_count"`
}

// fetchOllamaUsage calls the Ollama usage endpoint with the account's API
// key and returns the parsed snapshot. The caller must not hold p.mu.
func fetchOllamaUsage(apiKey string) (*ollamaUsage, error) {
	resp, err := hostHTTPDo(pluginapi.HTTPRequest{
		Method: http.MethodGet,
		URL:    ollamaUsageURL,
		Headers: http.Header{
			"Authorization": []string{"Bearer " + apiKey},
			"Accept":        []string{"application/json"},
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama usage HTTP %d", resp.StatusCode)
	}
	var raw ollamaUsageResponse
	if errUnmarshal := json.Unmarshal(resp.Body, &raw); errUnmarshal != nil {
		return nil, fmt.Errorf("ollama usage parse: %w", errUnmarshal)
	}
	out := &ollamaUsage{
		MonthlyPercent: pctInt(raw.Limits.Monthly.Usage),
		WeeklyPercent:  -1,
		SessionPercent: -1,
		FetchedAt:      time.Now(),
	}
	if raw.Limits.Weekly != nil {
		out.WeeklyPercent = pctInt(raw.Limits.Weekly.Usage)
	}
	if raw.Limits.Session != nil {
		out.SessionPercent = pctInt(raw.Limits.Session.Usage)
	}
	for _, m := range raw.Limits.Monthly.Models {
		out.Models = append(out.Models, ollamaModelUsage{Name: m.Name, RequestCount: m.RequestCount})
	}
	return out, nil
}

// pctInt converts a 0..1 fraction to a rounded integer percentage; values
// outside the valid range are clamped.
func pctInt(fraction float64) int {
	if fraction <= 0 {
		return 0
	}
	if fraction >= 1 {
		return 100
	}
	return int(fraction*100 + 0.5)
}

// ollamaWindowMap maps the ollamaUsage snapshot onto the shared windowState
// keys so the scheduler's existing threshold check works unchanged.
var ollamaWindowMap = []struct {
	field string
	get   func(*ollamaUsage) int
}{
	{windowMonthly, func(u *ollamaUsage) int { return u.MonthlyPercent }},
	{windowWeekly, func(u *ollamaUsage) int { return u.WeeklyPercent }},
	{windowFiveHour, func(u *ollamaUsage) int { return u.SessionPercent }},
}

// applyOllamaUsage writes a fetched usage snapshot into the account's window
// states. Any window at or above the threshold is blocked until the next
// successful fetch below threshold (the upstream reset boundary is not part
// of the API response, so the block self-clears on the next poll).
func applyOllamaUsage(p *pool, acct *account, u *ollamaUsage) {
	now := time.Now()
	p.mu.Lock()
	st := p.stateFor(acct)
	st.DashboardRefreshedAt = now
	st.DashboardError = ""
	st.OllamaModels = u.Models
	for _, m := range ollamaWindowMap {
		w := st.Windows[m.field]
		pct := m.get(u)
		if pct < 0 {
			continue
		}
		w.UsagePercent = pct
		w.UpdatedAt = now
		// A fresh reading below threshold clears an earlier block: the
		// upstream window has reset or the quota was topped up.
		if pct < p.cfg.ThresholdPercent && w.BlockedUntil.After(now) {
			w.BlockedUntil = time.Time{}
		}
	}
	p.mu.Unlock()
}

// applyOllamaError records a polling failure without touching window state.
func applyOllamaError(p *pool, acct *account, message string) {
	p.mu.Lock()
	st := p.stateFor(acct)
	st.DashboardError = message
	p.mu.Unlock()
	hostLog("warn", "ollama usage refresh failed", map[string]any{"account": acct.Name, "error": message})
}

// refreshOllamaAccount polls the usage endpoint for one Ollama account.
// The pool lock must NOT be held; results are applied under the lock.
func refreshOllamaAccount(p *pool, acct *account) {
	p.mu.Lock()
	apiKey := acct.apiKey
	p.mu.Unlock()
	if apiKey == "" {
		applyOllamaError(p, acct, "api key not available")
		return
	}
	u, err := fetchOllamaUsage(apiKey)
	if err != nil {
		applyOllamaError(p, acct, err.Error())
		return
	}
	applyOllamaUsage(p, acct, u)
}
