package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"OpenCodeGoPool/internal/model"
)

// ollamaUsageURL is the account usage endpoint exposed by ollama.com. It is
// the same endpoint consumed by Kosello/ollama-cloud-watch; unlike the
// OpenCode dashboard it is reachable with the API key alone (Bearer), so no
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

// FetchOllamaUsage calls the Ollama usage endpoint with the account's API
// key and returns the parsed snapshot.
func (c *Client) FetchOllamaUsage(apiKey string) (*model.OllamaUsage, error) {
	req, err := http.NewRequest(http.MethodGet, ollamaUsageURL, nil)
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
		return nil, fmt.Errorf("ollama api key invalid (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var raw ollamaUsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse ollama usage: %w", err)
	}

	u := &model.OllamaUsage{
		MonthlyPercent: pctInt(raw.Limits.Monthly.Usage),
		ScrapedAt:      time.Now().UTC(),
	}
	for _, m := range raw.Limits.Monthly.Models {
		u.Models = append(u.Models, model.OllamaModelUsage{Name: m.Name, RequestCount: m.RequestCount})
	}
	return u, nil
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
