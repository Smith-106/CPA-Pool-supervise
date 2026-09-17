export interface Account {
  id: string;
  type: 'opencode' | 'ollama';
  email: string;
  cookie?: string;
  workspace_id: string;
  api_key?: string;
  status: 'active' | 'rate_limited' | 'error' | 'disabled';
  status_msg: string;
  limit_rolling?: number | null;
  limit_weekly?: number | null;
  limit_monthly?: number | null;
  limit_exceeded: boolean;
  created_at: string;
  updated_at: string;
}

export interface QuotaSnapshot {
  id: string;
  account_id: string;
  rolling_percent: number;
  rolling_status: string;
  rolling_reset_sec: number;
  weekly_percent: number;
  weekly_status: string;
  weekly_reset_sec: number;
  monthly_percent: number;
  monthly_status: string;
  monthly_reset_sec: number;
  scraped_at: string;
}

export interface OllamaModelUsage {
  name: string;
  request_count: number;
}

export interface OllamaUsage {
  id: string;
  account_id: string;
  monthly_percent: number;
  models?: OllamaModelUsage[];
  scraped_at: string;
}

export interface AccountWithQuota extends Account {
  quota?: QuotaSnapshot;
  ollama_usage?: OllamaUsage;
}

export interface UsageRecord {
  id: string;
  account_id: string;
  model: string;
  provider: string;
  inputTokens: number;
  outputTokens: number;
  reasoningTokens: number | null;
  cacheReadTokens: number;
  cost: number;
  timeCreated: string;
  plan: string;
  scraped_at: string;
}

export interface UsageDailySummary {
  date: string;
  model: string;
  provider: string;
  inputTokens: number;
  outputTokens: number;
  reasoningTokens: number;
  cacheReadTokens: number;
  cost: number;
}

export interface CPASyncLog {
  id: string;
  status: string;
  message: string;
  key_count: number;
  synced_at: string;
}

export interface CPASettings {
  endpoint: string;
  bearer_token: string;
  provider_name: string;
  base_url: string;
  models: string[];
}

export interface DashboardData {
  accounts: AccountWithQuota[];
  cpa_sync: CPASyncLog | null;
}
