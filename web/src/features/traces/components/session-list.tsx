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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ChevronLeft, ChevronRight } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCompactNumber } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'
import { listTapSessions } from '../api'
import { clientLabel, relativeTime } from '../lib'
import type { TapSessionListItem } from '../types'

const PAGE_SIZE = 20
const DAY_OPTIONS = ['1', '7', '30']
const CLIENT_OPTIONS = ['all', 'claude', 'codex', 'cursor', 'cli', 'other']

type Props = {
  selectedId: number | null
  onSelect: (id: number) => void
}

export function SessionList({ selectedId, onSelect }: Props) {
  const { t, i18n } = useTranslation()
  const [days, setDays] = useState('7')
  const [client, setClient] = useState('all')
  const [search, setSearch] = useState('')
  const [debounced, setDebounced] = useState('')
  const [includeAux, setIncludeAux] = useState(false)
  const [page, setPage] = useState(1)

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebounced(search.trim())
      setPage(1)
    }, 350)
    return () => window.clearTimeout(timer)
  }, [search])

  const { data, isLoading } = useQuery({
    queryKey: ['tap-sessions', page, days, client, debounced, includeAux],
    queryFn: async () => {
      const result = requireServerSuccess(
        await listTapSessions({
          p: page,
          page_size: PAGE_SIZE,
          days: Number(days),
          client: client === 'all' ? undefined : client,
          q: debounced || undefined,
          include_aux: includeAux || undefined,
        })
      )
      if (!result.data) throw new Error(t('Failed to load call traces'))
      return result.data
    },
    placeholderData: (previous) => previous,
  })

  const items = data?.items ?? []
  const total = data?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const dayItems = DAY_OPTIONS.map((value) => ({
    value,
    label: t('Last {{days}} days', { days: value }),
  }))
  const clientItems = CLIENT_OPTIONS.map((value) => {
    let label = clientLabel(value, value)
    if (value === 'all') label = t('All clients')
    else if (value === 'other') label = t('Other clients')
    return { value, label }
  })

  return (
    <div className='bg-muted/20 flex min-h-0 flex-col gap-3 rounded-xl border p-3'>
      <div className='flex flex-col gap-2'>
        <Select
          value={days}
          onValueChange={(value) => {
            if (!value) return
            setDays(value)
            setPage(1)
          }}
          items={dayItems}
        >
          <SelectTrigger aria-label={t('Time range')} className='h-8 text-xs'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {dayItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={client}
          onValueChange={(value) => {
            if (!value) return
            setClient(value)
            setPage(1)
          }}
          items={clientItems}
        >
          <SelectTrigger aria-label={t('Client')} className='h-8 text-xs'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {clientItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Search model, fingerprint or message')}
          className='h-8 text-xs'
        />

        <label className='text-muted-foreground flex cursor-pointer items-center gap-1.5 text-[11px]'>
          <Checkbox
            checked={includeAux}
            onCheckedChange={(checked) => {
              setIncludeAux(checked === true)
              setPage(1)
            }}
            aria-label={t('Include aux calls')}
          />
          {t('Include aux calls')}
        </label>
      </div>

      <div className='min-h-0 flex-1 space-y-1.5 overflow-auto'>
        {isLoading && items.length === 0 && (
          <div className='space-y-1.5'>
            {[0, 1, 2, 3, 4].map((key) => (
              <Skeleton key={key} className='h-12 w-full' />
            ))}
          </div>
        )}

        {!isLoading && items.length === 0 && (
          <p className='text-muted-foreground px-1 py-6 text-center text-xs'>
            {t('No sessions in this range')}
          </p>
        )}

        {items.map((session) => (
          <SessionCard
            key={session.id}
            session={session}
            selected={session.id === selectedId}
            onSelect={onSelect}
            locale={i18n.language}
          />
        ))}
      </div>

      <div className='text-muted-foreground flex items-center justify-between text-[11px]'>
        <Button
          variant='ghost'
          size='icon-xs'
          disabled={page <= 1}
          onClick={() => setPage((prev) => Math.max(1, prev - 1))}
          aria-label={t('Previous page')}
        >
          <ChevronLeft />
        </Button>
        <span>
          {t('{{total}} total · page {{page}} / {{pages}}', {
            total,
            page,
            pages,
          })}
        </span>
        <Button
          variant='ghost'
          size='icon-xs'
          disabled={page >= pages}
          onClick={() => setPage((prev) => Math.min(pages, prev + 1))}
          aria-label={t('Next page')}
        >
          <ChevronRight />
        </Button>
      </div>
    </div>
  )
}

function SessionCard({
  session,
  selected,
  onSelect,
  locale,
}: {
  session: TapSessionListItem
  selected: boolean
  onSelect: (id: number) => void
  locale: string
}) {
  const { t } = useTranslation()
  const subtitle = [
    session.model || '—',
    t('{{turns}} turns', { turns: session.actual_turns }),
    session.fingerprint.slice(0, 6),
  ].join(' · ')

  return (
    <button
      type='button'
      onClick={() => onSelect(session.id)}
      className={cn(
        'w-full rounded-lg border px-2.5 py-2 text-left transition-colors',
        selected
          ? 'border-primary/40 bg-primary/10'
          : 'border-transparent hover:bg-muted'
      )}
    >
      <div className='flex items-center justify-between gap-2'>
        <span className='truncate text-xs font-medium'>{subtitle}</span>
        <span className='flex shrink-0 items-center gap-1'>
          {session.kind !== 'main' && (
            <Badge className='bg-amber-500/15 text-[10px] text-amber-700 dark:text-amber-300'>
              {t('aux')}
            </Badge>
          )}
          {session.kind === 'main' && session.aux_count > 0 && (
            <span className='text-muted-foreground text-[10px]'>
              {t('+{{n}} aux', { n: session.aux_count })}
            </span>
          )}
          <Badge variant='outline' className='text-[10px]'>
            {clientLabel(session.client, t('unknown'))}
          </Badge>
        </span>
      </div>
      <div className='text-muted-foreground mt-1 flex items-center justify-between text-[11px]'>
        <span>{relativeTime(session.updated_at, locale)}</span>
        <span>
          {t('In {{prompt}} / out {{completion}}', {
            prompt: formatCompactNumber(session.prompt_tokens),
            completion: formatCompactNumber(session.completion_tokens),
          })}
        </span>
      </div>
    </button>
  )
}
