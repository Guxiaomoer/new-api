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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Loader2 } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { requireServerSuccess } from '@/lib/server-error-message'
import { getTapDiff, getTapTurn } from '../api'
import type { TapDiff, TapRecon, TapTurnDetail } from '../types'
import {
  contentToText,
  messagesOf,
  parseJson,
  prettyJson,
  stableKey,
  type MessageLike,
} from './payload-utils'

function PreBlock({ children }: { children: React.ReactNode }) {
  return (
    <pre className='bg-muted/40 max-h-[60dvh] overflow-auto rounded-lg p-3 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap'>
      {children}
    </pre>
  )
}

type ViewKey = 'client' | 'upstream' | 'response' | 'client_response'

type RequestDoc = {
  headers?: Record<string, string>
  body?: unknown
  upstream_headers?: Record<string, string>
  upstream_body?: unknown
}

type Props = {
  turnId: number | null
  prevTurnId: number | null
  onOpenChange: (open: boolean) => void
}

export function TurnDialog({ turnId, prevTurnId, onOpenChange }: Props) {
  const { t } = useTranslation()
  const [view, setView] = useState<ViewKey>('client')

  const turnQuery = useQuery({
    queryKey: ['tap-turn', turnId],
    enabled: turnId !== null,
    queryFn: async () => {
      const result = requireServerSuccess(await getTapTurn(turnId as number))
      if (!result.data) throw new Error(t('Turn not found'))
      return result.data
    },
  })

  const turn = turnQuery.data?.turn
  const doc = turn ? parseJson<RequestDoc>(turn.request_json) : null

  return (
    <Dialog open={turnId !== null} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-5xl'>
        <DialogHeader>
          <DialogTitle className='pr-8'>
            {t('Turn #{{seq}}', { seq: turn?.seq ?? '' })}
            {turn?.truncated && (
              <Badge variant='warning' className='ml-2'>
                {t('Truncated')}
              </Badge>
            )}
          </DialogTitle>
        </DialogHeader>

        {!turn ? (
          <div className='text-muted-foreground flex items-center gap-2 py-10 text-sm'>
            <Loader2 className='size-4 animate-spin' />
            {t('Loading...')}
          </div>
        ) : (
          <div className='space-y-3'>
            <TurnMeta turn={turn} />

            {turn.truncated && (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'This turn was clipped to the body limit. Diff falls back to digests.'
                )}
              </p>
            )}

            <Tabs value={view} onValueChange={(v) => setView(v as ViewKey)}>
              <TabsList variant='line'>
                <TabsTrigger value='client'>{t('Client request')}</TabsTrigger>
                <TabsTrigger value='upstream' disabled={!doc?.upstream_body}>
                  {t('Upstream request')}
                </TabsTrigger>
                <TabsTrigger value='response'>
                  {t('Upstream response')}
                </TabsTrigger>
                <TabsTrigger
                  value='client_response'
                  disabled={!turn.client_response_text}
                >
                  {t('Client response')}
                </TabsTrigger>
              </TabsList>

              <TabsContent value='client'>
                <PayloadPane
                  raw={JSON.stringify(
                    { headers: doc?.headers, body: doc?.body },
                    null,
                    2
                  )}
                  body={doc?.body}
                />
              </TabsContent>
              <TabsContent value='upstream'>
                <PayloadPane
                  raw={JSON.stringify(
                    { headers: doc?.upstream_headers, body: doc?.upstream_body },
                    null,
                    2
                  )}
                  body={doc?.upstream_body}
                />
              </TabsContent>
              <TabsContent value='response'>
                <PayloadPane
                  raw={turn.response_text}
                  recon={parseJson<TapRecon>(turn.recon_json)}
                />
              </TabsContent>
              <TabsContent value='client_response'>
                <PayloadPane
                  raw={turn.client_response_text}
                  recon={turnQuery.data?.client_recon ?? null}
                />
              </TabsContent>
            </Tabs>

            {prevTurnId && (
              <DiffSection
                beforeId={prevTurnId}
                afterId={turn.id}
                fromDigestHint={turn.truncated}
              />
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

function TurnMeta({ turn }: { turn: TapTurnDetail }) {
  const { t } = useTranslation()
  return (
    <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs'>
      <span>{turn.model || '-'}</span>
      {turn.mapped_model && turn.mapped_model !== turn.model && (
        <span>→ {turn.mapped_model}</span>
      )}
      <span>{turn.protocol}</span>
      <span>{turn.stream ? t('stream') : t('non-stream')}</span>
      <span>HTTP {turn.status}</span>
      <span>{turn.latency_ms} ms</span>
      {turn.first_token_ms > 0 && <span>TTFB {turn.first_token_ms} ms</span>}
      <span>
        {turn.prompt_tokens} / {turn.completion_tokens}
      </span>
      {turn.cache_hit_tokens > 0 && (
        <span>
          {t('Cache hit')} {turn.cache_hit_tokens}
        </span>
      )}
      {turn.error && <span className='text-destructive'>{turn.error}</span>}
    </div>
  )
}

function PayloadPane({
  raw,
  body,
  recon,
}: {
  raw: string
  body?: unknown
  recon?: TapRecon | null
}) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'raw' | 'recon'>('raw')
  const messages = body !== undefined ? messagesOf(body) : []
  const hasRecon = Boolean(
    recon && (recon.text || recon.reasoning || recon.tool_calls?.length)
  )

  let content: React.ReactNode
  if (mode === 'raw') {
    content = raw ? (
      <PreBlock>{prettyJson(raw)}</PreBlock>
    ) : (
      <Hint text={t('Empty')} />
    )
  } else if (body !== undefined) {
    content =
      messages.length > 0 ? (
        <MessageList messages={messages} />
      ) : (
        <Hint text={t('No messages in this body')} />
      )
  } else if (hasRecon && recon) {
    content = <ReconView recon={recon} />
  } else {
    content = <Hint text={t('Nothing to reconstruct')} />
  }

  return (
    <div className='space-y-2'>
      <div className='flex items-center gap-1'>
        <Button
          size='sm'
          variant={mode === 'raw' ? 'secondary' : 'ghost'}
          onClick={() => setMode('raw')}
        >
          {t('Raw')}
        </Button>
        <Button
          size='sm'
          variant={mode === 'recon' ? 'secondary' : 'ghost'}
          onClick={() => setMode('recon')}
        >
          {t('Reconstructed')}
        </Button>
      </div>

      {content}
    </div>
  )
}

function MessageList({ messages }: { messages: MessageLike[] }) {
  return (
    <div className='max-h-[60dvh] space-y-2 overflow-auto'>
      {messages.map((msg) => {
        const text = contentToText(msg.content)
        return (
          <div
            key={stableKey(`${String(msg.role)}-${text}`)}
            className='border-border/60 rounded-lg border px-3 py-2'
          >
            <p className='text-muted-foreground mb-1 text-xs font-medium'>
              {String(msg.role ?? 'unknown')} · {text.length}
            </p>
            <p className='text-xs break-words whitespace-pre-wrap'>
              {text.slice(0, 2000) || '-'}
            </p>
          </div>
        )
      })}
    </div>
  )
}

function ReconView({ recon }: { recon: TapRecon }) {
  const { t } = useTranslation()
  return (
    <div className='max-h-[60dvh] space-y-3 overflow-auto'>
      {recon.reasoning && <Block title={t('Reasoning')} text={recon.reasoning} />}
      {recon.text && <Block title={t('Text')} text={recon.text} />}
      {recon.tool_calls?.map((call) => (
        <div key={stableKey(`${call.name}-${call.arguments}`)}>
          <p className='text-muted-foreground mb-1 text-xs font-medium'>
            {t('Tool call')}: {call.name}
          </p>
          <PreBlock>{prettyJson(call.arguments)}</PreBlock>
        </div>
      ))}
    </div>
  )
}

function Hint({ text }: { text: string }) {
  return <p className='text-muted-foreground text-xs'>{text}</p>
}

function Block({ title, text }: { title: string; text: string }) {
  return (
    <div>
      <p className='text-muted-foreground mb-1 text-xs font-medium'>{title}</p>
      <p className='text-xs break-words whitespace-pre-wrap'>{text}</p>
    </div>
  )
}

function DiffSection({
  beforeId,
  afterId,
  fromDigestHint,
}: {
  beforeId: number
  afterId: number
  fromDigestHint: boolean
}) {
  const { t } = useTranslation()
  const [enabled, setEnabled] = useState(false)
  const diffQuery = useQuery({
    queryKey: ['tap-diff', beforeId, afterId],
    enabled,
    queryFn: async () => {
      const result = requireServerSuccess(await getTapDiff(beforeId, afterId))
      return result.data ?? null
    },
  })

  return (
    <div className='border-border/60 rounded-lg border'>
      <div className='flex items-center gap-2 px-3 py-2'>
        <Button
          size='sm'
          variant='outline'
          onClick={() => setEnabled((prev) => !prev)}
        >
          {enabled ? t('Hide diff') : t('Compare with previous')}
        </Button>
        {enabled && fromDigestHint && (
          <span className='text-muted-foreground text-xs'>
            {t('Compared by digest')}
          </span>
        )}
        {enabled && diffQuery.isFetching && (
          <Loader2 className='text-muted-foreground size-3.5 animate-spin' />
        )}
      </div>
      {enabled && diffQuery.data && (
        <div className='border-border/60 space-y-2 border-t px-3 py-2 text-xs'>
          <DiffBody diff={diffQuery.data} />
        </div>
      )}
    </div>
  )
}

function DiffBody({ diff }: { diff: TapDiff }) {
  const { t } = useTranslation()
  const summary = diff.summary
  return (
    <>
      <div className='flex flex-wrap gap-x-4 gap-y-1'>
        <span>
          {t('Messages')}: {diff.messages.before_total} →{' '}
          {diff.messages.after_total}
        </span>
        <span>
          {t('Added')}: {summary.added}
        </span>
        <span>
          {t('Changed')}: {summary.changed}
        </span>
        <span>
          {t('Removed')}: {summary.removed}
        </span>
        <span>
          {t('Params')}: {summary.params}
        </span>
        <span>
          {t('Tools')}: {summary.tools}
        </span>
      </div>

      {diff.params.length > 0 && (
        <div>
          <p className='text-muted-foreground font-medium'>
            {t('Changed params')}
          </p>
          {diff.params.map((item) => (
            <p key={item.key}>
              {item.key}: {item.before} → {item.after}
            </p>
          ))}
        </div>
      )}

      {diff.messages.added.length > 0 && (
        <div>
          <p className='text-muted-foreground font-medium'>{t('Added')}</p>
          {diff.messages.added.map((item) => (
            <p key={item.index} className='break-words'>
              #{item.index} {item.role}: {item.preview}
            </p>
          ))}
        </div>
      )}

      {diff.messages.changed.length > 0 && (
        <div>
          <p className='text-muted-foreground font-medium'>{t('Changed')}</p>
          {diff.messages.changed.map((item) => (
            <p key={item.index} className='break-words'>
              #{item.index} {item.role}: {item.before} → {item.after}
            </p>
          ))}
        </div>
      )}

      {diff.messages.removed.length > 0 && (
        <div>
          <p className='text-muted-foreground font-medium'>{t('Removed')}</p>
          {diff.messages.removed.map((item) => (
            <p key={item.index} className='break-words'>
              #{item.index} {item.role}: {item.preview}
            </p>
          ))}
        </div>
      )}

      {(diff.tools.added.length > 0 ||
        diff.tools.removed.length > 0 ||
        diff.tools.changed.length > 0) && (
        <div>
          <p className='text-muted-foreground font-medium'>{t('Tools')}</p>
          {diff.tools.added.length > 0 && (
            <p>+ {diff.tools.added.join(', ')}</p>
          )}
          {diff.tools.removed.length > 0 && (
            <p>- {diff.tools.removed.join(', ')}</p>
          )}
          {diff.tools.changed.map((item) => (
            <p key={item.name} className='break-words'>
              {item.name}: {item.before} → {item.after}
            </p>
          ))}
        </div>
      )}
    </>
  )
}
