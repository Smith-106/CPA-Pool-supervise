package model

import "time"

type Account struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"` // "opencode" or "ollama"; empty is treated as "opencode"
	Email         string    `json:"email"`
	Cookie        string    `json:"cookie,omitempty"`
	WorkspaceID   string    `json:"workspace_id"`
	APIKey        string    `json:"api_key,omitempty"`
	Status        string    `json:"status"`
	StatusMsg     string    `json:"status_msg"`
	LimitRolling  *int      `json:"limit_rolling,omitempty"`
	LimitWeekly   *int      `json:"limit_weekly,omitempty"`
	LimitMonthly  *int      `json:"limit_monthly,omitempty"`
	LimitExceeded bool      `json:"limit_exceeded"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AccountWithQuota struct {
	Account
	Quota *QuotaSnapshot `json:"quota,omitempty"`
}
