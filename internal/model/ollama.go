package model

import "time"

// OllamaModelUsage is one model's request count inside the monthly quota.
type OllamaModelUsage struct {
	Name         string `json:"name"`
	RequestCount int    `json:"request_count"`
}

// OllamaUsage is the parsed snapshot of https://ollama.com/api/usage for one
// API key. Only the fields needed for quota protection are decoded; unknown
// fields are ignored so the monitor keeps working when Ollama adds data.
type OllamaUsage struct {
	ID            string             `json:"id"`
	AccountID     string             `json:"account_id"`
	MonthlyPercent int               `json:"monthly_percent"`
	Models        []OllamaModelUsage `json:"models,omitempty"`
	ScrapedAt     time.Time          `json:"scraped_at"`
}
