import type { Account, CPASettings, CPASyncLog, DashboardData, QuotaSnapshot, UsageDailySummary, UsageRecord } from './types';

const BASE = '/api/v1';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  if (res.status === 401) {
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`);
  }
  return data as T;
}

export const api = {
  auth: {
    login: (password: string) =>
      request<{ status: string }>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ password }),
      }),
    check: () => request<{ authenticated: boolean }>('/auth/check'),
  },

  dashboard: () => request<DashboardData>('/dashboard'),

  accounts: {
    list: () => request<Account[]>('/accounts'),
    get: (id: string) => request<Account>(`/accounts/${id}`),
    create: (data: { type: 'opencode' | 'ollama'; email: string; cookie?: string; workspace_id?: string; api_key: string; limit_rolling?: number | null; limit_weekly?: number | null; limit_monthly?: number | null }) =>
      request<Account>('/accounts', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: string, data: Partial<Account>) =>
      request<Account>(`/accounts/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: string) =>
      request<{ status: string }>(`/accounts/${id}`, { method: 'DELETE' }),
    toggleStatus: (id: string) =>
      request<{ status: string }>(`/accounts/${id}/status`, { method: 'PATCH' }),
    quota: (id: string, limit = 50) =>
      request<QuotaSnapshot[]>(`/accounts/${id}/quota?limit=${limit}`),
    usage: (id: string, limit = 100, offset = 0) =>
      request<UsageRecord[]>(`/accounts/${id}/usage?limit=${limit}&offset=${offset}`),
    usageDailyHistory: (id: string) =>
      request<UsageDailySummary[]>(`/accounts/${id}/usage/daily`),
    refresh: (id: string) =>
      request<{ status: string }>(`/accounts/${id}/refresh`, { method: 'POST' }),
  },

  refreshAll: () => request<{ status: string }>('/refresh', { method: 'POST' }),

  sync: {
    trigger: () => request<{ status: string }>('/sync/cpa', { method: 'POST' }),
    status: () => request<CPASyncLog | null>('/sync/cpa/status'),
  },

  settings: {
    getCPA: () => request<CPASettings>('/settings/cpa'),
    updateCPA: (data: CPASettings) =>
      request<CPASettings>('/settings/cpa', { method: 'PUT', body: JSON.stringify(data) }),
  },
};
