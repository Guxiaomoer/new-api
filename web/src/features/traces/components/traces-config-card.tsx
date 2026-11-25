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
import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Loader2, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { formatCompactNumber } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { getTapConfig, pruneTap, updateTapConfig } from '../api'
import type { TapConfig } from '../types'

export function TracesConfigCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['tap-config'],
    queryFn: async () => {
      const result = requireServerSuccess(await getTapConfig())
      if (!result.data) throw new Error(t('Failed to load call trace settings'))
      return result.data
    },
  })

  const [form, setForm] = useState<TapConfig | null>(null)
  useEffect(() => {
    if (data?.config) setForm(data.config)
  }, [data])

  const saveMutation = useMutation({
    mutationFn: async (next: TapConfig) =>
      requireServerSuccess(await updateTapConfig(next)),
    onSuccess: () => {
      toast.success(t('Call trace settings saved'))
      queryClient.invalidateQueries({ queryKey: ['tap-config'] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const pruneMutation = useMutation({
    mutationFn: async () => {
      const result = requireServerSuccess(await pruneTap())
      return result.data ?? { deleted: 0 }
    },
    onSuccess: (result) => {
      toast.success(
        t('Deleted {{count}} expired sessions', { count: result.deleted })
      )
      queryClient.invalidateQueries({ queryKey: ['tap-config'] })
      queryClient.invalidateQueries({ queryKey: ['tap-sessions'] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  if (isLoading || !form) {
    return (
      <div className='border-border/60 text-muted-foreground flex items-center gap-2 rounded-xl border px-4 py-3 text-sm'>
        <Loader2 className='size-4 animate-spin' />
        {t('Loading...')}
      </div>
    )
  }

  const stats = data?.stats
  const patch = (next: Partial<TapConfig>) =>
    setForm((prev) => (prev ? { ...prev, ...next } : prev))

  return (
    <div className='border-border/60 bg-card space-y-3 rounded-xl border px-4 py-3'>
      <div className='flex flex-wrap items-center gap-x-6 gap-y-3'>
        <div className='flex items-center gap-2'>
          <Switch
            id='tap-enabled'
            size='sm'
            checked={form.enabled}
            onCheckedChange={(checked) => patch({ enabled: checked })}
          />
          <Label htmlFor='tap-enabled' className='cursor-pointer text-sm'>
            {t('Record calls')}
          </Label>
        </div>

        <div className='flex items-center gap-2'>
          <Label htmlFor='tap-max-body' className='text-muted-foreground text-xs'>
            {t('Max body')}
          </Label>
          <Input
            id='tap-max-body'
            type='number'
            className='h-8 w-28'
            value={Math.round(form.max_body / 1024)}
            onChange={(e) =>
              patch({ max_body: Number(e.target.value || 0) * 1024 })
            }
          />
          <span className='text-muted-foreground text-xs'>KB</span>
        </div>

        <div className='flex items-center gap-2'>
          <Label
            htmlFor='tap-retention'
            className='text-muted-foreground text-xs'
          >
            {t('Keep days')}
          </Label>
          <Input
            id='tap-retention'
            type='number'
            className='h-8 w-20'
            value={form.retention_days}
            onChange={(e) =>
              patch({ retention_days: Number(e.target.value || 1) })
            }
          />
        </div>

        <div className='flex items-center gap-2'>
          <Label
            htmlFor='tap-max-sessions'
            className='text-muted-foreground text-xs'
          >
            {t('Max sessions')}
          </Label>
          <Input
            id='tap-max-sessions'
            type='number'
            className='h-8 w-24'
            value={form.max_sessions}
            onChange={(e) =>
              patch({ max_sessions: Number(e.target.value || 10) })
            }
          />
        </div>

        <div className='ml-auto flex items-center gap-2'>
          <Button
            size='sm'
            variant='outline'
            onClick={() => pruneMutation.mutate()}
            disabled={pruneMutation.isPending}
          >
            <Trash2 className='size-4' />
            {t('Prune now')}
          </Button>
          <Button
            size='sm'
            onClick={() => saveMutation.mutate(form)}
            disabled={saveMutation.isPending}
          >
            {t('Save')}
          </Button>
        </div>
      </div>

      {stats && (
        <div className='text-muted-foreground flex flex-wrap gap-x-5 gap-y-1 text-xs'>
          <span>
            {t('Sessions')}: {formatCompactNumber(stats.sessions)}
          </span>
          <span>
            {t('Turns')}: {formatCompactNumber(stats.turns)}
          </span>
          <span>
            {t('Prompt tokens')}: {formatCompactNumber(stats.prompt_tokens)}
          </span>
          <span>
            {t('Completion tokens')}:{' '}
            {formatCompactNumber(stats.completion_tokens)}
          </span>
        </div>
      )}
    </div>
  )
}
