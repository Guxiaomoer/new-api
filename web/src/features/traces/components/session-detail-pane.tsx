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
import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Download, Trash2 } from 'lucide-react'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestamp } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'
import { deleteTapSession, downloadTapExport, getTapSession } from '../api'
import { clientLabel, clientVersion, relativeTime } from '../lib'
import type { TapSession, TapTurn } from '../types'
import { TurnTabs } from './turn-tabs'

// 模块级常量：查询未返回时保持引用稳定，避免 effect 每渲染重跑。
const EMPTY_TURNS: TapTurn[] = []

type Props = {
  sessionId: number
  onSelectSession: (id: number) => void
  onDeleted: () => void
}

export function SessionDetailPane({
  sessionId,
  onSelectSession,
  onDeleted,
}: Props) {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const [selectedTurnId, setSelectedTurnId] = useState<number | null>(null)
  const [newCount, setNewCount] = useState(0)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const ackedSeq = useRef(0)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['tap-session', sessionId],
    queryFn: async () => {
      const result = requireServerSuccess(await getTapSession(sessionId))
      if (!result.data) throw new Error(t('Session not found'))
      return result.data
    },
  })

  useEffect(() => {
    ackedSeq.current = 0
    setNewCount(0)
    setSelectedTurnId(null)
  }, [sessionId])

  const turns = data?.turns ?? EMPTY_TURNS

  useEffect(() => {
    if (turns.length === 0) {
      setNewCount(0)
      return
    }
    const lastSeq = turns.at(-1)?.seq ?? 0
    if (ackedSeq.current === 0) {
      ackedSeq.current = lastSeq
      setNewCount(0)
      return
    }
    if (lastSeq > ackedSeq.current) {
      setNewCount(turns.filter((turn) => turn.seq > ackedSeq.current).length)
    }
  }, [turns])

  const deleteMutation = useMutation({
    mutationFn: async () => requireServerSuccess(await deleteTapSession(sessionId)),
    onSuccess: () => {
      toast.success(t('Session deleted'))
      queryClient.invalidateQueries({ queryKey: ['tap-sessions'] })
      queryClient.invalidateQueries({ queryKey: ['tap-config'] })
      setConfirmDelete(false)
      onDeleted()
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const exportMutation = useMutation({
    mutationFn: () => downloadTapExport(sessionId),
    onError: (error: Error) => toast.error(error.message),
  })

  if (isLoading) {
    return (
      <div className='space-y-2 rounded-xl border p-3'>
        <Skeleton className='h-6 w-1/3' />
        <Skeleton className='h-4 w-2/3' />
        <Skeleton className='h-40 w-full' />
      </div>
    )
  }

  if (isError || !data) {
    return (
      <EmptyState
        bordered
        title={t('Failed to load this session')}
        description={t('Refresh the page or pick another session.')}
      />
    )
  }

  const { session, aux } = data
  const activeTurn = resolveTurn(turns, selectedTurnId)
  const prevTurn = previousTurn(turns, activeTurn)
  const lastSeq = turns.at(-1)?.seq ?? 0

  const acknowledge = () => {
    ackedSeq.current = lastSeq
    setNewCount(0)
  }

  const viewLatest = () => {
    acknowledge()
    setSelectedTurnId(turns.at(-1)?.id ?? null)
  }

  return (
    <div className='flex min-h-0 flex-col gap-3 rounded-xl border p-3'>
      <header className='space-y-1'>
        <div className='flex flex-wrap items-start justify-between gap-2'>
          <div className='min-w-0'>
            <h3 className='truncate text-sm font-semibold'>
              {session.model || '—'}
              <span className='text-muted-foreground ml-2 text-xs font-normal'>
                {t('{{turns}} turns', { turns: session.turns })} ·{' '}
                {session.fingerprint.slice(0, 6)}
              </span>
            </h3>
            <div className='text-muted-foreground mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px]'>
              <span>
                {clientLabel(session.client, t('unknown'))}
                {clientVersion(session.client_ua)
                  ? ` ${clientVersion(session.client_ua)}`
                  : ''}
              </span>
              <span>{session.protocol || '—'}</span>
              <span>{session.key_name || `#${session.key_id}`}</span>
              <span title={formatTimestamp(session.started_at)}>
                {relativeTime(session.started_at, i18n.language)}
                {session.updated_at !== session.started_at
                  ? ` → ${relativeTime(session.updated_at, i18n.language)}`
                  : ''}
              </span>
            </div>
          </div>
          <div className='flex shrink-0 items-center gap-1'>
            <Button
              variant='outline'
              size='xs'
              disabled={exportMutation.isPending}
              onClick={() => exportMutation.mutate()}
            >
              <Download />
              {t('Export')}
            </Button>
            <Button
              variant='outline'
              size='icon-xs'
              aria-label={t('Delete session')}
              onClick={() => setConfirmDelete(true)}
            >
              <Trash2 />
            </Button>
          </div>
        </div>

        {(aux.length > 0 || (session.kind !== 'main' && session.parent_id > 0)) && (
          <div className='flex flex-wrap items-center gap-1 text-[11px]'>
            {session.kind !== 'main' && session.parent_id > 0 && (
              <button
                type='button'
                onClick={() => onSelectSession(session.parent_id)}
                className='text-muted-foreground hover:bg-muted rounded-full border px-1.5 text-[10px]'
              >
                {t('Aux call of #{{id}}', { id: session.parent_id })}
              </button>
            )}
            {aux.length > 0 && (
              <>
                <span className='text-muted-foreground'>
                  {t('{{n}} aux calls', { n: aux.length })}
                </span>
                {aux.map((item) => (
                  <button
                    key={item.id}
                    type='button'
                    onClick={() => onSelectSession(item.id)}
                    className='hover:bg-muted rounded-full border px-1.5 text-[10px]'
                  >
                    {auxLabel(item)}
                  </button>
                ))}
              </>
            )}
          </div>
        )}

        <Timeline turns={turns} />
      </header>

      {turns.length === 0 ? (
        <EmptyState
          title={t('No turns captured yet')}
          description={t('Send another request from this client to record a turn.')}
        />
      ) : (
        <div className='flex min-h-0 flex-1 flex-col gap-2'>
          <div className='flex flex-wrap items-center gap-1'>
            {turns.map((turn) => (
              <button
                key={turn.id}
                type='button'
                onClick={() => {
                  setSelectedTurnId(turn.id)
                  if (turn.seq >= lastSeq) acknowledge()
                }}
                title={turn.path}
                className={cn(
                  'rounded-full border px-1.5 py-0.5 text-[10px] transition-colors',
                  activeTurn?.id === turn.id
                    ? 'border-primary/50 bg-primary/10'
                    : 'hover:bg-muted',
                  turn.error ? 'text-red-600' : 'text-muted-foreground'
                )}
              >
                #{turn.seq}
                {turn.stream ? ' · stream' : ''}
                {turn.truncated ? ' · ✂' : ''}
              </button>
            ))}
          </div>

          {newCount > 0 && (
            <button
              type='button'
              onClick={viewLatest}
              className='rounded-full border border-primary/40 bg-primary/10 px-2 py-1 text-[11px] text-primary'
            >
              {t('{{n}} new turns, click to view the latest', { n: newCount })}
            </button>
          )}

          {activeTurn && (
            <TurnTabs
              key={activeTurn.id}
              turnId={activeTurn.id}
              prevTurnId={prevTurn?.id ?? null}
            />
          )}
        </div>
      )}

      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title={t('Delete this session')}
        desc={t(
          'All recorded turns of this session will be removed. This cannot be undone.'
        )}
        confirmText={t('Delete')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => deleteMutation.mutate()}
      />
    </div>
  )
}

function resolveTurn(
  turns: TapTurn[],
  selectedId: number | null
): TapTurn | null {
  const hit = turns.find((turn) => turn.id === selectedId)
  if (hit) return hit
  return turns.at(-1) ?? null
}

function previousTurn(turns: TapTurn[], active: TapTurn | null): TapTurn | null {
  if (!active) return null
  const index = turns.findIndex((turn) => turn.id === active.id)
  return index > 0 ? turns[index - 1] : null
}

function auxLabel(session: TapSession) {
  return `#${session.id}`
}
function Timeline({ turns }: { turns: TapTurn[] }) {
  const { t } = useTranslation()
  if (turns.length === 0) return null
  const paths: string[] = []
  for (const turn of turns) {
    const label = turn.path || '—'
    if (paths.at(-1) !== label) paths.push(label)
  }
  const seen = new Map<string, number>()
  const steps = paths.map((path) => {
    const nth = (seen.get(path) ?? 0) + 1
    seen.set(path, nth)
    return { path, key: `${path}#${nth}` }
  })
  return (
    <div className='text-muted-foreground flex flex-wrap items-center gap-1 text-[10px]'>
      <span>
        {t('Timeline')} · {t('{{turns}} turns', { turns: turns.length })}
      </span>
      {steps.map((step, index) => (
        <span key={step.key} className='flex items-center gap-1'>
          {index > 0 && <span>→</span>}
          <span className='rounded border px-1 font-mono'>{step.path}</span>
        </span>
      ))}
      {turns.some((turn) => turn.error) && (
        <Badge className='bg-red-600/15 text-[10px] text-red-600'>
          {t('has errors')}
        </Badge>
      )}
    </div>
  )
}
