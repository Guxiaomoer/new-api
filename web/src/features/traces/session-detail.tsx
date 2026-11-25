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
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ArrowLeft, Download, Loader2, Trash2 } from 'lucide-react'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatCompactNumber, formatTimestamp } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { deleteTapSession, downloadTapExport, getTapSession } from './api'
import type { TapTurn } from './types'
import { TurnDialog } from './components/turn-dialog'

export function TraceSessionDetail({ id }: { id: number }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<{ turn: TapTurn; prev?: TapTurn } | null>(
    null
  )

  const { data, isLoading } = useQuery({
    queryKey: ['tap-session', id],
    queryFn: async () => {
      const result = requireServerSuccess(await getTapSession(id))
      if (!result.data) throw new Error(t('Session not found'))
      return result.data
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async () => requireServerSuccess(await deleteTapSession(id)),
    onSuccess: () => {
      toast.success(t('Session deleted'))
      queryClient.invalidateQueries({ queryKey: ['tap-sessions'] })
      navigate({ to: '/traces' })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const exportMutation = useMutation({
    mutationFn: () => downloadTapExport(id),
    onError: (error: Error) => toast.error(error.message),
  })

  const session = data?.session
  const turns = data?.turns ?? []
  const aux = data?.aux ?? []

  const handleDelete = () => {
    if (!window.confirm(t('Delete this session and all its turns?'))) return
    deleteMutation.mutate()
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Call trace')} #{id}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          size='sm'
          variant='ghost'
          onClick={() => navigate({ to: '/traces' })}
        >
          <ArrowLeft className='size-4' />
          {t('Back')}
        </Button>
        <Button
          size='sm'
          variant='outline'
          onClick={() => exportMutation.mutate()}
          disabled={exportMutation.isPending}
        >
          <Download className='size-4' />
          {t('Export HTML')}
        </Button>
        <Button
          size='sm'
          variant='outline'
          onClick={handleDelete}
          disabled={deleteMutation.isPending}
        >
          <Trash2 className='size-4' />
          {t('Delete')}
        </Button>
      </SectionPageLayout.Actions>

      <SectionPageLayout.Content>
        {isLoading || !session ? (
          <div className='text-muted-foreground flex items-center gap-2 py-10 text-sm'>
            <Loader2 className='size-4 animate-spin' />
            {t('Loading...')}
          </div>
        ) : (
          <div className='space-y-4'>
            <div className='border-border/60 grid gap-x-6 gap-y-2 rounded-xl border px-4 py-3 text-sm sm:grid-cols-2 lg:grid-cols-3'>
              <Fact label={t('Client')} value={session.client || 'unknown'} />
              <Fact label='UA' value={session.client_ua || '-'} />
              <Fact label={t('Protocol')} value={session.protocol} />
              <Fact label={t('Model')} value={session.model || '-'} />
              <Fact label={t('Key')} value={session.key_name || '-'} />
              <Fact
                label={t('Time range')}
                value={`${formatTimestamp(session.started_at)} → ${formatTimestamp(session.updated_at)}`}
              />
              <Fact
                label={t('Tokens')}
                value={formatCompactNumber(
                  session.prompt_tokens + session.completion_tokens
                )}
              />
              <Fact label={t('Fingerprint')} value={session.fingerprint} />
            </div>

            {session.first_user && (
              <div className='border-border/60 rounded-xl border px-4 py-3 text-sm'>
                <p className='text-muted-foreground mb-1 text-xs'>
                  {t('Opening message')}
                </p>
                <p className='break-words whitespace-pre-wrap'>
                  {session.first_user}
                </p>
              </div>
            )}

            {aux.length > 0 && (
              <div className='flex flex-wrap items-center gap-2 text-sm'>
                <span className='text-muted-foreground text-xs'>
                  {t('Aux calls')}:
                </span>
                {aux.map((item) => (
                  <Link key={item.id} to='/traces/$id' params={{ id: String(item.id) }}>
                    <Badge variant='outline'>
                      #{item.id} {item.kind}
                    </Badge>
                  </Link>
                ))}
              </div>
            )}

            <div className='border-border/60 overflow-x-auto rounded-xl border'>
              <table className='w-full text-left text-xs'>
                <thead className='bg-muted/30 text-muted-foreground'>
                  <tr>
                    <th className='px-3 py-2 font-medium'>#</th>
                    <th className='px-3 py-2 font-medium'>{t('Time')}</th>
                    <th className='px-3 py-2 font-medium'>{t('Status')}</th>
                    <th className='px-3 py-2 font-medium'>{t('Model')}</th>
                    <th className='px-3 py-2 font-medium'>{t('Tokens')}</th>
                    <th className='px-3 py-2 font-medium'>{t('Latency')}</th>
                    <th className='px-3 py-2 font-medium'>TTFB</th>
                    <th className='px-3 py-2 font-medium'>{t('Error')}</th>
                  </tr>
                </thead>
                <tbody>
                  {turns.map((turn, index) => (
                    <tr
                      key={turn.id}
                      className='hover:bg-muted/30 cursor-pointer border-t'
                      onClick={() =>
                        setSelected({
                          turn,
                          prev: index > 0 ? turns[index - 1] : undefined,
                        })
                      }
                    >
                      <td className='px-3 py-2'>{turn.seq}</td>
                      <td className='px-3 py-2 whitespace-nowrap'>
                        {formatTimestamp(turn.ts)}
                      </td>
                      <td className='px-3 py-2'>
                        <span
                          className={
                            turn.status >= 400 ? 'text-destructive' : undefined
                          }
                        >
                          {turn.status}
                        </span>
                        <span className='text-muted-foreground ml-1'>
                          {turn.stream ? 'SSE' : ''}
                        </span>
                      </td>
                      <td className='px-3 py-2'>
                        {turn.model || '-'}
                        {turn.mapped_model && turn.mapped_model !== turn.model && (
                          <span className='text-muted-foreground'>
                            {' '}
                            → {turn.mapped_model}
                          </span>
                        )}
                      </td>
                      <td className='px-3 py-2 whitespace-nowrap'>
                        {turn.prompt_tokens} / {turn.completion_tokens}
                      </td>
                      <td className='px-3 py-2 whitespace-nowrap'>
                        {turn.latency_ms} ms
                      </td>
                      <td className='px-3 py-2 whitespace-nowrap'>
                        {turn.first_token_ms > 0 ? `${turn.first_token_ms} ms` : '-'}
                      </td>
                      <td className='text-destructive max-w-[220px] truncate px-3 py-2'>
                        {turn.error || ''}
                      </td>
                    </tr>
                  ))}
                  {turns.length === 0 && (
                    <tr>
                      <td
                        colSpan={8}
                        className='text-muted-foreground px-3 py-6 text-center text-xs'
                      >
                        {t('No turns in this session')}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </SectionPageLayout.Content>

      <TurnDialog
        turnId={selected?.turn.id ?? null}
        prevTurnId={selected?.prev?.id ?? null}
        onOpenChange={(open) => {
          if (!open) setSelected(null)
        }}
      />
    </SectionPageLayout>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className='min-w-0'>
      <p className='text-muted-foreground text-xs'>{label}</p>
      <p className='truncate' title={value}>
        {value || '-'}
      </p>
    </div>
  )
}
