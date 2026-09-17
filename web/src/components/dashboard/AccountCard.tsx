import { useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { RefreshCw, BarChart3 } from 'lucide-react'

import type { AccountWithQuota } from '@/api/types'
import { api } from '@/api/client'
import { accountStatusMeta } from '@/lib/status'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { StatusBadge } from '@/components/common/StatusBadge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { QuotaBar } from './QuotaBar'

export function AccountCard({ account }: { account: AccountWithQuota }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const refreshMut = useMutation({
    mutationFn: () => api.accounts.refresh(account.id),
    onSuccess: () => {
      setTimeout(() => queryClient.invalidateQueries({ queryKey: ['dashboard'] }), 3000)
    },
  })

  const toggleMut = useMutation({
    mutationFn: () => api.accounts.toggleStatus(account.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })

  const meta = accountStatusMeta(account.status)

  return (
    <Card className="p-5 transition duration-200 hover:-translate-y-0.5 hover:shadow-whisper">
      <div className="mb-4 flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-foreground">{account.email}</p>
          <div className="mt-1.5 flex items-center gap-1.5">
            <StatusBadge tone={meta.tone} label={meta.label} />
            {account.type === 'ollama' && (
              <span className="rounded-full bg-blue-500/10 px-2 py-0.5 text-[10px] font-medium text-blue-600 dark:text-blue-400">Ollama</span>
            )}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => navigate(`/accounts/${account.id}/usage`)}
              >
                <BarChart3 />
              </Button>
            </TooltipTrigger>
            <TooltipContent>View usage</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => refreshMut.mutate()}
                disabled={refreshMut.isPending}
              >
                <RefreshCw className={refreshMut.isPending ? 'animate-spin' : ''} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Refresh</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Switch
                checked={account.status !== 'disabled'}
                disabled={toggleMut.isPending}
                onCheckedChange={() => toggleMut.mutate()}
              />
            </TooltipTrigger>
            <TooltipContent>{account.status === 'disabled' ? 'Enable' : 'Disable'}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {account.status === 'error' && account.status_msg && (
        <p className="mb-4 break-all rounded-lg bg-destructive/5 px-3 py-2 text-xs text-destructive">
          {account.status_msg}
        </p>
      )}

      {account.limit_exceeded && (
        <div className="mb-3">
          <StatusBadge tone="warning" label="Removed from CPA" withDot />
        </div>
      )}

      {account.type === 'ollama' ? (
        account.ollama_usage ? (
          <div className="space-y-3">
            <QuotaBar
              label="Monthly"
              percent={account.ollama_usage.monthly_percent}
              resetSec={0}
              limit={account.limit_monthly}
            />
            <p className="mt-3 text-[10px] text-muted-foreground">
              Updated: {new Date(account.ollama_usage.scraped_at).toLocaleString()}
            </p>
          </div>
        ) : (
          <p className="text-xs italic text-muted-foreground">No usage data yet</p>
        )
      ) : account.quota ? (
        <div className="space-y-3">
          <QuotaBar
            label="Rolling"
            percent={account.quota.rolling_percent}
            resetSec={account.quota.rolling_reset_sec}
            limit={account.limit_rolling}
          />
          <QuotaBar
            label="Weekly"
            percent={account.quota.weekly_percent}
            resetSec={account.quota.weekly_reset_sec}
            limit={account.limit_weekly}
          />
          <QuotaBar
            label="Monthly"
            percent={account.quota.monthly_percent}
            resetSec={account.quota.monthly_reset_sec}
            limit={account.limit_monthly}
          />
        </div>
      ) : (
        <p className="text-xs italic text-muted-foreground">No quota data yet</p>
      )}

      {account.type !== 'ollama' && account.quota && (
        <p className="mt-3 text-[10px] text-muted-foreground">
          Updated: {new Date(account.quota.scraped_at).toLocaleString()}
        </p>
      )}
    </Card>
  )
}
