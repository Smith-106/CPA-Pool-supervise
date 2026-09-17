import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Pencil, Trash2 } from 'lucide-react'

import { api } from '@/api/client'
import type { Account } from '@/api/types'
import { accountStatusMeta } from '@/lib/status'
import { notifyError, notifySuccess } from '@/lib/errors'
import { PageHeader } from '@/components/common/PageHeader'
import { EmptyState } from '@/components/common/EmptyState'
import { ErrorState } from '@/components/common/ErrorState'
import { TableSkeleton } from '@/components/common/LoadingState'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
import { StatusBadge } from '@/components/common/StatusBadge'
import { CopyButton } from '@/components/common/CopyButton'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { AccountForm } from './AccountForm'

export function AccountsPage() {
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Account | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<Account | undefined>()
  const queryClient = useQueryClient()

  const { data: accounts = [], isLoading, isError, error, refetch } = useQuery({
    queryKey: ['accounts'],
    queryFn: api.accounts.list,
    select: (data) => data ?? [],
  })

  const deleteMut = useMutation({
    mutationFn: api.accounts.delete,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accounts'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      notifySuccess('Account deleted')
      setDeleteTarget(undefined)
    },
    onError: (err) => notifyError(err),
  })

  const toggleMut = useMutation({
    mutationFn: api.accounts.toggleStatus,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accounts'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    },
    onError: (err) => notifyError(err),
  })

  return (
    <div className="space-y-6">
      <PageHeader
        title="Accounts"
        description="Manage your OpenCode Go accounts"
        actions={
          <Button
            onClick={() => {
              setEditing(undefined)
              setFormOpen(true)
            }}
          >
            <Plus />
            Add Account
          </Button>
        }
      />

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="p-4">
              <TableSkeleton />
            </div>
          ) : isError ? (
            <div className="p-4">
              <ErrorState error={error} onRetry={() => refetch()} />
            </div>
          ) : accounts.length === 0 ? (
            <EmptyState title="No accounts yet" description='Click "Add Account" to start.' />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Type</TableHead>
                <TableHead>Email</TableHead>
                  <TableHead>Workspace</TableHead>
                  <TableHead>API Key</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {accounts.map((acc) => {
                  const meta = accountStatusMeta(acc.status)
                  return (
                    <TableRow key={acc.id}>
                      <TableCell className="font-medium text-foreground">{acc.email}</TableCell>
                      <TableCell>
                        <CopyButton
                          value={acc.workspace_id}
                          label={`${acc.workspace_id.slice(0, 15)}...`}
                          variant="ghost"
                          className="font-mono text-xs text-muted-foreground"
                        />
                      </TableCell>
                      <TableCell>
                        {acc.api_key ? (
                          <CopyButton
                            value={acc.api_key}
                            label={`${acc.api_key.slice(0, 10)}...`}
                            variant="ghost"
                            className="font-mono text-xs text-muted-foreground"
                          />
                        ) : (
                          <span className="text-xs text-muted-foreground">-</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap items-center gap-1.5">
                          <StatusBadge tone={meta.tone} label={meta.label} />
                          {acc.limit_exceeded && (
                            <StatusBadge tone="warning" label="Limit" withDot />
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8"
                            onClick={() => {
                              setEditing(acc)
                              setFormOpen(true)
                            }}
                            title="Edit"
                          >
                            <Pencil />
                          </Button>
                          <Switch
                            checked={acc.status !== 'disabled'}
                            disabled={toggleMut.isPending}
                            onCheckedChange={() => toggleMut.mutate(acc.id)}
                            title={acc.status === 'disabled' ? 'Enable' : 'Disable'}
                          />
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-destructive hover:bg-destructive/10 hover:text-destructive"
                            onClick={() => setDeleteTarget(acc)}
                            title="Delete"
                          >
                            <Trash2 />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <AccountForm
        open={formOpen}
        onOpenChange={(open) => {
          setFormOpen(open)
          if (!open) setEditing(undefined)
        }}
        account={editing}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(undefined)}
        title="Delete account?"
        description={
          deleteTarget && (
            <>
              This will permanently delete <strong>{deleteTarget.email}</strong>.
            </>
          )
        }
        confirmText="Delete"
        destructive
        loading={deleteMut.isPending}
        onConfirm={() => deleteTarget && deleteMut.mutate(deleteTarget.id)}
      />
    </div>
  )
}
