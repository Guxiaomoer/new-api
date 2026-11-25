/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useCallback, useEffect, useState } from 'react'
import {
  useIsFetching,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { RefreshCw, Settings2, Trash2 } from 'lucide-react'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { formatCompactNumber } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import {
  clearTap,
  getTapConfig,
  pruneTap,
  updateTapConfig,
} from './api'
import { SessionDetailPane } from './components/session-detail-pane'
import { SessionList } from './components/session-list'
import { formatSize } from './lib'
import type { TapConfig } from './types'

const INTERVALS = ['5', '10', '15', '30', '60']
const LS_AUTO = 'tap-auto-refresh'
const LS_INTERVAL = 'tap-refresh-interval'

export function Traces({ initialSessionId }: { initialSessionId?: number }) {
  const { t } = useTranslation()

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Call Traces')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <TracesWorkspace initialSessionId={initialSessionId} />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function TracesWorkspace({
  initialSessionId,
}: {
  initialSessionId?: number
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [sessionId, setSessionId] = useState<number | null>(
    initialSessionId ?? null
  )
  const [confirmClear, setConfirmClear] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(
    () => localStorage.getItem(LS_AUTO) === '1'
  )
  const [intervalSec, setIntervalSec] = useState(
    () => localStorage.getItem(LS_INTERVAL) || '15'
  )
  const [lastUpdated, setLastUpdated] = useState(0)
  const [now, setNow] = useState(() => Date.now())

  const { data, isLoading } = useQuery({
    queryKey: ['tap-config'],
    queryFn: async () => {
      const result = requireServerSuccess(await getTapConfig())
      if (!result.data) throw new Error(t('Failed to load call trace settings'))
      return result.data
    },
    refetchOnWindowFocus: false,
  })

  const fetching = useIsFetching({
    predicate: (query) => String(query.queryKey[0]).startsWith('tap-'),
  })

  useEffect(() => {
    if (!lastUpdated && !autoRefresh) return
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [lastUpdated, autoRefresh])

  const refresh = useCallback(async () => {
    await queryClient.invalidateQueries({
      predicate: (query) => String(query.queryKey[0]).startsWith('tap-'),
      refetchType: 'active',
    })
    setLastUpdated(Date.now())
    setNow(Date.now())
  }, [queryClient])

  useEffect(() => {
    if (!autoRefresh || !data?.config.enabled) return
    const timer = window.setInterval(
      () => void refresh(),
      Number(intervalSec) * 1000
    )
    return () => window.clearInterval(timer)
  }, [autoRefresh, intervalSec, data?.config.enabled, refresh])

  const saveMutation = useMutation({
    mutationFn: async (next: TapConfig) =>
      requireServerSuccess(await updateTapConfig(next)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tap-config'] })
      queryClient.invalidateQueries({ queryKey: ['tap-sessions'] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const clearMutation = useMutation({
    mutationFn: async () => requireServerSuccess(await clearTap()),
    onSuccess: () => {
      toast.success(t('All call traces cleared'))
      setConfirmClear(false)
      setSessionId(null)
      void refresh()
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const config = data?.config
  const stats = data?.stats

  const patch = (next: Partial<TapConfig>) => {
    if (!config) return
    saveMutation.mutate({ ...config, ...next })
  }

  const secondsAgo = lastUpdated
    ? Math.max(0, Math.round((now - lastUpdated) / 1000))
    : null

  let updatedText = ''
  if (fetching > 0) updatedText = t('Updating…')
  else if (secondsAgo !== null) {
    updatedText = t('Updated {{s}}s ago', { s: secondsAgo })
  }

  return (
    <div className='flex h-full min-h-0 flex-col gap-3'>
      <div className='flex flex-wrap items-center gap-x-4 gap-y-2'>
        <div className='flex items-center gap-2'>
          <Switch
            id='tap-enabled'
            size='sm'
            checked={config?.enabled ?? false}
            disabled={isLoading || saveMutation.isPending}
            onCheckedChange={(checked) => patch({ enabled: checked })}
          />
          <Label htmlFor='tap-enabled' className='cursor-pointer text-sm'>
            {config?.enabled ? t('Recording calls') : t('Record calls')}
          </Label>
        </div>

        <Button
          variant='outline'
          size='xs'
          onClick={() => void refresh()}
          disabled={fetching > 0}
        >
          <RefreshCw className={fetching > 0 ? 'animate-spin' : undefined} />
          {t('Refresh')}
        </Button>

        <div className='flex items-center gap-2 text-xs'>
          <Switch
            id='tap-auto'
            size='sm'
            checked={autoRefresh}
            onCheckedChange={(checked) => {
              setAutoRefresh(checked)
              localStorage.setItem(LS_AUTO, checked ? '1' : '0')
            }}
          />
          <Label htmlFor='tap-auto' className='cursor-pointer text-xs'>
            {t('Auto refresh')}
          </Label>
          <span className='text-muted-foreground text-[11px]'>
            {updatedText}
          </span>
        </div>

        <Popover>
          <PopoverTrigger
            render={
              <Button variant='outline' size='xs'>
                <Settings2 />
                {t('Refresh settings')}
              </Button>
            }
          />
          <PopoverContent align='start' className='w-64 space-y-3 p-3'>
            <div className='flex items-center justify-between gap-2'>
              <span className='text-xs font-medium'>{t('Auto refresh')}</span>
              <Switch
                size='sm'
                checked={autoRefresh}
                onCheckedChange={(checked) => {
                  setAutoRefresh(checked)
                  localStorage.setItem(LS_AUTO, checked ? '1' : '0')
                }}
              />
            </div>
            <div className='space-y-1'>
              <span className='text-xs font-medium'>{t('Refresh interval')}</span>
              <Select
                value={intervalSec}
                onValueChange={(value) => {
                  if (!value) return
                  setIntervalSec(value)
                  localStorage.setItem(LS_INTERVAL, value)
                }}
                items={INTERVALS.map((value) => ({
                  value,
                  label: t('{{s}} seconds', { s: value }),
                }))}
              >
                <SelectTrigger className='h-8 text-xs'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {INTERVALS.map((value) => (
                    <SelectItem key={value} value={value}>
                      {t('{{s}} seconds', { s: value })}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <p className='text-muted-foreground text-[11px]'>
              {t('Auto refresh only re-reads stored traces; it does not send requests.')}
            </p>
          </PopoverContent>
        </Popover>

        <Popover>
          <PopoverTrigger
            render={
              <Button variant='outline' size='xs'>
                <Settings2 />
                {t('Storage settings')}
              </Button>
            }
          />
          <PopoverContent align='start' className='w-72 space-y-3 p-3'>
            <div className='grid grid-cols-2 gap-2'>
              <div className='space-y-1'>
                <Label className='text-muted-foreground text-[11px]'>
                  {t('Max body')} (KB)
                </Label>
                <Input
                  type='number'
                  className='h-8 text-xs'
                  defaultValue={Math.round((config?.max_body ?? 0) / 1024)}
                  onBlur={(event) =>
                    patch({ max_body: Number(event.target.value || 0) * 1024 })
                  }
                />
              </div>
              <div className='space-y-1'>
                <Label className='text-muted-foreground text-[11px]'>
                  {t('Keep days')}
                </Label>
                <Input
                  type='number'
                  className='h-8 text-xs'
                  defaultValue={config?.retention_days ?? 7}
                  onBlur={(event) =>
                    patch({ retention_days: Number(event.target.value || 1) })
                  }
                />
              </div>
              <div className='space-y-1'>
                <Label className='text-muted-foreground text-[11px]'>
                  {t('Max sessions')}
                </Label>
                <Input
                  type='number'
                  className='h-8 text-xs'
                  defaultValue={config?.max_sessions ?? 2000}
                  onBlur={(event) =>
                    patch({ max_sessions: Number(event.target.value || 10) })
                  }
                />
              </div>
              <div className='flex items-end'>
                <Button
                  variant='outline'
                  size='xs'
                  className='w-full'
                  onClick={() =>
                    pruneTap()
                      .then((result) => {
                        const deleted =
                          requireServerSuccess(result).data?.deleted ?? 0
                        toast.success(
                          t('Deleted {{count}} expired sessions', {
                            count: deleted,
                          })
                        )
                        void refresh()
                      })
                      .catch((error: Error) => toast.error(error.message))
                  }
                >
                  <Trash2 />
                  {t('Prune now')}
                </Button>
              </div>
            </div>
            <p className='text-muted-foreground text-[11px]'>
              {t('Changes save when you leave a field.')}
            </p>
          </PopoverContent>
        </Popover>

        <Button
          variant='outline'
          size='xs'
          className='text-destructive'
          disabled={!stats?.sessions}
          onClick={() => setConfirmClear(true)}
        >
          <Trash2 />
          {t('Clear')}
        </Button>
      </div>

      {stats && (
        <p className='text-muted-foreground text-[11px]'>
          {t(
            '{{sessions}} sessions / {{turns}} turns stored · cumulative tokens in {{prompt}} / out {{completion}} · keep {{days}} days, {{size}} per request',
            {
              sessions: formatCompactNumber(stats.sessions),
              turns: formatCompactNumber(stats.turns),
              prompt: formatCompactNumber(stats.prompt_tokens),
              completion: formatCompactNumber(stats.completion_tokens),
              days: config?.retention_days ?? 0,
              size: formatSize(config?.max_body ?? 0),
            }
          )}
        </p>
      )}

      {!isLoading && config && !config.enabled ? (
        <EmptyState
          bordered
          title={t('Call tracing is off')}
          description={t(
            'Turn it on to record the request and response of every text generation call. Credentials are masked before storage.'
          )}
          action={
            <Button
              size='sm'
              disabled={saveMutation.isPending}
              onClick={() => patch({ enabled: true })}
            >
              {t('Turn on call tracing')}
            </Button>
          }
        />
      ) : (
        <div className='grid min-h-0 flex-1 gap-4 lg:grid-cols-[320px_minmax(0,1fr)]'>
          <SessionList selectedId={sessionId} onSelect={setSessionId} />
          <div className='min-h-0 min-w-0 overflow-auto'>
            {sessionId === null ? (
              <EmptyState
                bordered
                title={t('Pick a session on the left')}
                description={t(
                  'Each session groups consecutive turns that share the same client, protocol and first message.'
                )}
              />
            ) : (
              <SessionDetailPane
                key={sessionId}
                sessionId={sessionId}
                onSelectSession={setSessionId}
                onDeleted={() => setSessionId(null)}
              />
            )}
          </div>
        </div>
      )}

      <ConfirmDialog
        open={confirmClear}
        onOpenChange={setConfirmClear}
        title={t('Clear all call traces')}
        desc={t(
          'Every stored session and turn will be deleted. This cannot be undone.'
        )}
        confirmText={t('Clear')}
        destructive
        isLoading={clearMutation.isPending}
        handleConfirm={() => clearMutation.mutate()}
      />
    </div>
  )
}
