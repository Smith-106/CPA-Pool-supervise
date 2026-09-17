import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import type { Account } from '@/api/types'
import { api } from '@/api/client'
import { notifyError, notifySuccess } from '@/lib/errors'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'

const limitField = z.preprocess(
  (v) => {
    if (v === '' || v == null) return undefined
    const n = Number(v)
    return n === 0 ? undefined : n
  },
  z.number().int().min(1, '最小 1').max(100, '最大 100').optional(),
)

const formSchema = z.object({
  type: z.enum(['opencode', 'ollama']).default('opencode'),
  email: z.string().min(1, 'Email is required'),
  cookie: z.string().optional(),
  workspace_id: z.string().optional(),
  api_key: z.string().optional(),
  limit_rolling: limitField,
  limit_weekly: limitField,
  limit_monthly: limitField,
})

type FormValues = z.infer<typeof formSchema>

interface AccountFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  account?: Account
}

export function AccountForm({ open, onOpenChange, account }: AccountFormProps) {
  const queryClient = useQueryClient()

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { type: 'opencode', email: '', cookie: '', workspace_id: '', api_key: '', limit_rolling: undefined, limit_weekly: undefined, limit_monthly: undefined },
  })

  useEffect(() => {
    if (open) {
      form.reset({
        type: (account?.type as 'opencode' | 'ollama') ?? 'opencode',
        email: account?.email ?? '',
        cookie: account?.cookie ?? '',
        workspace_id: account?.workspace_id ?? '',
        api_key: account?.api_key ?? '',
        limit_rolling: account?.limit_rolling ?? undefined,
        limit_weekly: account?.limit_weekly ?? undefined,
        limit_monthly: account?.limit_monthly ?? undefined,
      })
    }
  }, [open, account, form])

  const mutation = useMutation({
    mutationFn: (data: FormValues) => {
      const payload = {
        ...data,
        api_key: data.api_key ?? '',
        // Always include limit fields — null explicitly clears an existing limit.
        limit_rolling: data.limit_rolling ?? null,
        limit_weekly: data.limit_weekly ?? null,
        limit_monthly: data.limit_monthly ?? null,
      }
      return account ? api.accounts.update(account.id, payload) : api.accounts.create(payload)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      queryClient.invalidateQueries({ queryKey: ['accounts'] })
      notifySuccess(account ? 'Account updated' : 'Account created')
      onOpenChange(false)
    },
    onError: (err) => notifyError(err),
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{account ? 'Edit Account' : 'Add Account'}</DialogTitle>
        </DialogHeader>

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((values) => {
              if (values.type === 'ollama' && !values.api_key) {
                form.setError('api_key', { message: 'Ollama API Key is required' })
                return
              }
              if (values.type === 'opencode' && (!values.cookie || !values.workspace_id)) {
                if (!values.cookie) form.setError('cookie', { message: 'Cookie is required' })
                if (!values.workspace_id) form.setError('workspace_id', { message: 'Workspace ID is required' })
                return
              }
              mutation.mutate(values)
            })}
            className="space-y-4"
          >
            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Type</FormLabel>
                  <FormControl>
                    <select
                      className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm"
                      {...field}
                    >
                      <option value="opencode">OpenCode Go</option>
                      <option value="ollama">Ollama</option>
                    </select>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="email"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Email</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {form.watch('type') === 'opencode' && (
            <FormField
              control={form.control}
              name="cookie"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Cookie</FormLabel>
                  <FormControl>
                    <Textarea
                      className="h-24 resize-none font-mono"
                      placeholder="oc_locale=zh; auth=Fe26.2**..."
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            )}

            {form.watch('type') === 'opencode' && (
            <FormField
              control={form.control}
              name="workspace_id"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Workspace ID</FormLabel>
                  <FormControl>
                    <Input
                      className="font-mono"
                      placeholder="wrk_01KKA8GAS09F1MW95ANMT8222P"
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            )}

            <FormField
              control={form.control}
              name="api_key"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{form.watch('type') === 'ollama' ? 'Ollama API Key' : 'API Key'}</FormLabel>
                  <FormControl>
                    <Input className="font-mono" placeholder="sk-..." {...field} />
                  </FormControl>
                  <FormDescription>Used for CPA sync. Leave empty if not needed.</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="space-y-3 rounded-lg border p-3">
              <p className="text-xs font-medium text-muted-foreground">Quota Limits (% — leave blank for no limit)</p>
              <div className="grid grid-cols-3 gap-3">
                {(
                  [
                    ...(form.watch('type') === 'opencode'
                      ? ([
                          { name: 'limit_rolling', label: 'Rolling (5h)' },
                          { name: 'limit_weekly', label: 'Weekly' },
                        ] as const)
                      : []),
                    { name: 'limit_monthly', label: 'Monthly' },
                  ] as const
                ).map(({ name, label }) => (
                  <FormField
                    key={name}
                    control={form.control}
                    name={name}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel className="text-xs">{label}</FormLabel>
                        <FormControl>
                          <Input
                            type="text"
                            inputMode="numeric"
                            placeholder="—"
                            {...field}
                            value={field.value ?? ''}
                            onChange={(e) => field.onChange(e.target.value)}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                ))}
              </div>
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={mutation.isPending}>
                {mutation.isPending ? 'Saving...' : account ? 'Update' : 'Create'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
