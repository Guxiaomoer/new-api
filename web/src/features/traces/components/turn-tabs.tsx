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
import { useEffect, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Loader2 } from 'lucide-react'

import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCompactNumber, formatCurrencyUSD } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'
import { getTapDiff, getTapTurn } from '../api'
import {
  asObject,
  bodyOf,
  formatDuration,
  headerPairs,
  prettyJson,
  requestDoc,
  rewrittenKeys,
} from '../lib'
import type {
  TapDescribe,
  TapDiff,
  TapRecon,
  TapTurnDetail,
} from '../types'
import { JsonView } from './json-view'

type TabKey = 'overview' | 'request' | 'response' | 'diff'
type ViewKey = 'client' | 'gateway'
type FormatKey = 'preview' | 'raw'

const TABS: TabKey[] = ['overview', 'request', 'response', 'diff']

type Props = {
  turnId: number
  prevTurnId: number | null
}

export function TurnTabs({ turnId, prevTurnId }: Props) {
  const { t } = useTranslation()
  const [tab, setTab] = useState<TabKey>('overview')
  const [view, setView] = useState<ViewKey>('client')
  const [format, setFormat] = useState<FormatKey>('preview')
  const [expandAll, setExpandAll] = useState(false)

  useEffect(() => {
    setTab('overview')
    setExpandAll(false)
  }, [turnId])

  const turnQuery = useQuery({
    queryKey: ['tap-turn', turnId],
    queryFn: async () => {
      const result = requireServerSuccess(await getTapTurn(turnId))
      if (!result.data) throw new Error(t('Turn not found'))
      return result.data
    },
  })

  const diffQuery = useQuery({
    queryKey: ['tap-diff', prevTurnId, turnId],
    enabled: tab === 'diff' && prevTurnId !== null,
    queryFn: async () => {
      const result = requireServerSuccess(
        await getTapDiff(prevTurnId as number, turnId)
      )
      return result.data ?? null
    },
  })

  const turn = turnQuery.data?.turn

  const locate = () => {
    setView('gateway')
    setFormat('preview')
    setExpandAll(false)
    setTab('request')
  }

  return (
    <div className='flex min-h-0 flex-col gap-2'>
      <div className='flex flex-wrap items-center justify-between gap-2 border-b pb-2'>
        <div className='flex flex-wrap gap-1'>
          {TABS.map((key) => (
            <button
              key={key}
              type='button'
              onClick={() => setTab(key)}
              className={cn(
                'rounded-full px-2.5 py-1 text-xs transition-colors',
                tab === key
                  ? 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:bg-muted'
              )}
            >
              {key === 'diff' ? t('Compare with previous') : t(TAB_LABELS[key])}
            </button>
          ))}
        </div>

        <div className='flex flex-wrap items-center gap-3'>
          <Toggle
            label={t('View')}
            value={view}
            options={[
              ['client', t('Client')],
              ['gateway', t('Gateway')],
            ]}
            onChange={(next) => setView(next as ViewKey)}
          />
          {(tab === 'request' || tab === 'response') && (
            <Toggle
              label={t('Format')}
              value={format}
              options={[
                ['preview', t('Preview')],
                ['raw', t('Raw')],
              ]}
              onChange={(next) => setFormat(next as FormatKey)}
            />
          )}
        </div>
      </div>

      <div className='min-h-0 flex-1 overflow-auto'>
        {!turn && (
          <div className='space-y-2'>
            {turnQuery.isError ? (
              <p className='text-muted-foreground text-xs'>
                {t('Failed to load this turn')}
              </p>
            ) : (
              <>
                <Skeleton className='h-10 w-full' />
                <Skeleton className='h-40 w-full' />
              </>
            )}
          </div>
        )}

        {turn && tab === 'overview' && (
          <OverviewTab
            turn={turn}
            describe={turnQuery.data?.describe ?? null}
            recon={turnQuery.data?.client_recon ?? null}
          />
        )}

        {turn && tab === 'request' && (
          <RequestTab
            turn={turn}
            view={view}
            format={format}
            expandAll={expandAll}
            onToggleExpand={() => setExpandAll((prev) => !prev)}
          />
        )}

        {turn && tab === 'response' && (
          <ResponseTab
            turn={turn}
            view={view}
            format={format}
            clientRecon={turnQuery.data?.client_recon ?? null}
            expandAll={expandAll}
            onToggleExpand={() => setExpandAll((prev) => !prev)}
          />
        )}

        {turn && tab === 'diff' && (
          <DiffTab
            diff={diffQuery.data ?? null}
            loading={diffQuery.isFetching}
            hasPrev={prevTurnId !== null}
            onLocate={locate}
          />
        )}
      </div>
    </div>
  )
}

const TAB_LABELS: Record<Exclude<TabKey, 'diff'>, string> = {
  overview: 'Overview',
  request: 'Request',
  response: 'Response',
}

function Toggle({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: Array<[string, string]>
  onChange: (value: string) => void
}) {
  return (
    <div className='flex items-center gap-1.5 text-[11px]'>
      <span className='text-muted-foreground'>{label}</span>
      <div className='flex items-center rounded-full border p-0.5'>
        {options.map(([key, text]) => (
          <button
            key={key}
            type='button'
            aria-pressed={value === key}
            onClick={() => onChange(key)}
            className={cn(
              'rounded-full px-2 py-0.5 transition-colors',
              value === key
                ? 'bg-foreground/10 font-medium'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {text}
          </button>
        ))}
      </div>
    </div>
  )
}

function OverviewTab({
  turn,
  describe,
  recon,
}: {
  turn: TapTurnDetail
  describe: TapDescribe | null
  recon: TapRecon | null
}) {
  const { t } = useTranslation()
  const facts: Array<[string, string]> = [
    [t('Model'), turn.model || '—'],
    [t('Mapped model'), turn.mapped_model || '—'],
    [t('Status'), String(turn.status ?? '—')],
    [t('Latency'), formatDuration(turn.latency_ms)],
    [t('First token'), formatDuration(turn.first_token_ms)],
    [t('Prompt tokens'), formatCompactNumber(turn.prompt_tokens)],
    [t('Completion tokens'), formatCompactNumber(turn.completion_tokens)],
    [t('Cache hit'), formatCompactNumber(turn.cache_hit_tokens)],
    [t('Cache miss'), formatCompactNumber(turn.cache_miss_tokens)],
    [t('Cost'), turn.cost > 0 ? formatCurrencyUSD(turn.cost) : '—'],
    [
      t('Structure'),
      describe
        ? t('{{messages}} messages / {{tools}} tools / system {{chars}} chars', {
            messages: describe.messages,
            tools: describe.tools,
            chars: describe.system_chars,
          })
        : '—',
    ],
  ]

  const toolNames = describe?.tool_names ?? []

  return (
    <div className='space-y-3'>
      {turn.error && (
        <p className='rounded-md border border-red-500/30 bg-red-500/5 px-2 py-1 text-xs text-red-500'>
          {turn.error}
        </p>
      )}

      <dl className='grid grid-cols-2 gap-x-4 gap-y-1.5 text-xs sm:grid-cols-3'>
        {facts.map(([label, value]) => (
          <div key={label} className='flex min-w-0 flex-col'>
            <dt className='text-muted-foreground text-[11px]'>{label}</dt>
            <dd className='truncate' title={value}>
              {value}
            </dd>
          </div>
        ))}
      </dl>

      {toolNames.length > 0 && (
        <div className='flex flex-wrap gap-1'>
          {toolNames.map((name) => (
            <Badge key={name} variant='outline' className='text-[10px]'>
              {name}
            </Badge>
          ))}
        </div>
      )}

      {recon?.text && <Block title={t('Reconstructed text')} text={recon.text} />}
      {recon?.reasoning && <Block title={t('Reasoning')} text={recon.reasoning} />}
      {recon?.tool_calls && recon.tool_calls.length > 0 && (
        <ToolCalls calls={recon.tool_calls} />
      )}
      {!recon && (
        <p className='text-muted-foreground text-xs'>
          {t('No reconstructed output')}
        </p>
      )}
    </div>
  )
}

function RequestTab({
  turn,
  view,
  format,
  expandAll,
  onToggleExpand,
}: {
  turn: TapTurnDetail
  view: ViewKey
  format: FormatKey
  expandAll: boolean
  onToggleExpand: () => void
}) {
  const { t } = useTranslation()
  const doc = requestDoc(turn)
  const clientBody = asObject(bodyOf(doc, 'body'))
  const upstreamBody = asObject(bodyOf(doc, 'upstream_body'))
  const headers =
    view === 'client' ? headerPairs(doc?.headers) : headerPairs(doc?.upstream_headers)
  const body = view === 'client' ? bodyOf(doc, 'body') : bodyOf(doc, 'upstream_body')
  const rewrites = view === 'gateway' ? rewrittenKeys(clientBody, upstreamBody) : []

  let bodyContent: ReactNode
  if (body === undefined) {
    bodyContent = (
      <p className='text-muted-foreground text-[11px]'>
        {t('Nothing captured for this side.')}
      </p>
    )
  } else if (format === 'preview') {
    bodyContent = <JsonView value={body} expandAll={expandAll} />
  } else {
    bodyContent = (
      <pre className='bg-muted/40 max-h-[60vh] overflow-auto rounded-md p-2 font-mono text-[11px] break-words whitespace-pre-wrap'>
        {prettyJson(body)}
      </pre>
    )
  }

  if (!doc) {
    return (
      <div className='space-y-2'>
        <p className='text-[11px] text-amber-600'>
          {t('The request body was truncated at the body limit; showing the raw text.')}
        </p>
        <pre className='bg-muted/40 max-h-[60vh] overflow-auto rounded-md p-2 font-mono text-[11px] break-words whitespace-pre-wrap'>
          {turn.request_json || '—'}
        </pre>
      </div>
    )
  }

  return (
    <div className='min-w-0 space-y-3'>
      {turn.truncated && (
        <p className='text-[11px] text-amber-600'>
          {t('Some payloads were truncated at the body limit.')}
        </p>
      )}

      <p className='text-muted-foreground text-[11px]'>
        {view === 'client'
          ? t('Headers and body exactly as the client sent them.')
          : t('Body after the gateway rewrote the model, parameters and streaming options.')}
      </p>

      {view === 'gateway' && (
        <div className='flex flex-wrap items-center gap-1 text-[11px]'>
          {rewrites.length > 0 ? (
            <>
              <span className='text-muted-foreground'>
                {t('Fields changed by the gateway:')}
              </span>
              {rewrites.map((key) => (
                <span key={key} className='rounded-full border px-1.5 text-[10px]'>
                  {key}
                </span>
              ))}
            </>
          ) : (
            <span className='text-muted-foreground'>
              {t('The gateway forwarded this body unchanged.')}
            </span>
          )}
        </div>
      )}

      <section className='min-w-0'>
        <h4 className='mb-1 text-xs font-medium'>
          {view === 'client' ? t('Client headers') : t('Upstream headers')}
        </h4>
        {headers.length === 0 ? (
          <p className='text-muted-foreground text-[11px]'>
            {t('No headers captured')}
          </p>
        ) : (
          <dl className='bg-muted/40 max-h-56 min-w-0 overflow-auto rounded-md p-2 text-[11px]'>
            {headers.map(([name, value]) => (
              <div key={name} className='flex min-w-0 gap-2 py-0.5'>
                <dt className='text-muted-foreground w-40 shrink-0 truncate font-mono'>
                  {name}
                </dt>
                <dd className='min-w-0 flex-1 font-mono break-words'>
                  {value || '—'}
                </dd>
              </div>
            ))}
          </dl>
        )}
        <p className='text-muted-foreground mt-1 flex items-center gap-2 text-[10px]'>
          {t('Credential headers are masked before storage.')}
          <CopyButton
            value={headers.map(([name, value]) => `${name}: ${value}`).join('\n')}
            size='icon'
            className='size-6'
          />
        </p>
      </section>

      <section className='min-w-0'>
        <div className='mb-1 flex flex-wrap items-center gap-2'>
          <h4 className='text-xs font-medium'>
            {view === 'client' ? t('Request body') : t('Upstream request body')}
          </h4>
          {format === 'preview' && body !== undefined && (
            <Button
              type='button'
              variant='outline'
              size='xs'
              className='text-muted-foreground rounded-full text-[10px]'
              onClick={onToggleExpand}
            >
              {expandAll ? t('Collapse all') : t('Expand all')}
            </Button>
          )}
          <CopyButton
            value={prettyJson(body)}
            size='icon'
            className='size-6'
          />
        </div>
        {bodyContent}
      </section>
    </div>
  )
}

function ResponseTab({
  turn,
  view,
  format,
  clientRecon,
  expandAll,
  onToggleExpand,
}: {
  turn: TapTurnDetail
  view: ViewKey
  format: FormatKey
  clientRecon: TapRecon | null
  expandAll: boolean
  onToggleExpand: () => void
}) {
  const { t } = useTranslation()
  const isClient = view === 'client'
  const raw = isClient ? turn.client_response_text : turn.response_text
  const passthrough = isClient && !turn.client_response_text
  let hint = t('Raw bytes as received from upstream.')
  if (isClient) {
    hint = passthrough
      ? t('The gateway passed the upstream bytes through unchanged.')
      : t('The gateway translated the upstream response into the client protocol.')
  }

  return (
    <div className='min-w-0 space-y-3'>
      <p className='text-muted-foreground text-[11px]'>{hint}</p>

      {raw ? (
        <section className='min-w-0'>
          <div className='mb-1 flex flex-wrap items-center gap-2'>
            <h4 className='text-xs font-medium'>
              {isClient ? t('Client response') : t('Upstream response')}
            </h4>
            <Button
              type='button'
              variant='outline'
              size='xs'
              className='text-muted-foreground rounded-full text-[10px]'
              onClick={onToggleExpand}
            >
              {expandAll ? t('Collapse all') : t('Expand all')}
            </Button>
            <CopyButton value={raw} size='icon' className='size-6' />
          </div>
          {format === 'raw' ? (
            <pre className='bg-muted/40 max-h-[60vh] overflow-auto rounded-md p-2 font-mono text-[11px] break-words whitespace-pre-wrap'>
              {raw}
            </pre>
          ) : (
            <details className='text-xs'>
              <summary className='text-muted-foreground cursor-pointer'>
                {isClient ? t('Raw client stream') : t('Raw upstream stream')}
              </summary>
              <pre className='bg-muted/40 mt-1 max-h-[50vh] overflow-auto rounded-md p-2 font-mono text-[11px] break-words whitespace-pre-wrap'>
                {raw}
              </pre>
            </details>
          )}
          {format === 'preview' && <ReconBlocks recon={clientRecon} empty />}
        </section>
      ) : (
        <ReconBlocks recon={clientRecon} empty />
      )}
    </div>
  )
}

function ReconBlocks({ recon, empty }: { recon: TapRecon | null; empty?: boolean }) {
  const { t } = useTranslation()
  const hasBody =
    recon && (recon.text || recon.reasoning || (recon.tool_calls?.length ?? 0) > 0)
  if (!hasBody) {
    if (!empty) return null
    return (
      <p className='text-muted-foreground text-xs'>
        {t('Nothing captured for this side.')}
      </p>
    )
  }
  return (
    <div className='space-y-3'>
      {recon?.text && <Block title={t('Reconstructed text')} text={recon.text} />}
      {recon?.reasoning && <Block title={t('Reasoning')} text={recon.reasoning} />}
      {recon?.tool_calls && recon.tool_calls.length > 0 && (
        <ToolCalls calls={recon.tool_calls} />
      )}
    </div>
  )
}

function DiffTab({
  diff,
  loading,
  hasPrev,
  onLocate,
}: {
  diff: TapDiff | null
  loading: boolean
  hasPrev: boolean
  onLocate: () => void
}) {
  const { t } = useTranslation()

  if (!hasPrev) {
    return (
      <p className='text-muted-foreground text-xs'>
        {t('This is the first turn; nothing to compare.')}
      </p>
    )
  }

  if (!diff) {
    return loading ? (
      <Skeleton className='h-32 w-full' />
    ) : (
      <div className='text-muted-foreground flex items-center gap-2 text-xs'>
        <Loader2 className='size-3.5 animate-spin' />
        {t('Loading...')}
      </div>
    )
  }

  const summary = diff.summary
  const empty =
    summary.params === 0 &&
    !diff.system.changed &&
    summary.added === 0 &&
    summary.removed === 0 &&
    summary.changed === 0 &&
    summary.tools === 0

  if (empty) {
    return (
      <div className='space-y-1'>
        <p className='text-muted-foreground text-xs'>
          {t('No differences in this turn.')}
        </p>
        {diff.from_digest && (
          <p className='text-muted-foreground text-[11px]'>
            {t('The earlier turn was truncated, so this comparison uses the stored digest.')}
          </p>
        )}
      </div>
    )
  }

  return (
    <div className='space-y-4 text-xs'>
      {diff.from_digest && (
        <p className='rounded-md border border-amber-500/30 bg-amber-500/5 px-2 py-1.5 text-[11px] text-amber-700 dark:text-amber-300'>
          {t('The earlier turn was truncated, so this comparison uses the stored digest.')}
        </p>
      )}

      {diff.params.length > 0 && (
        <section>
          <h4 className='mb-1 font-medium'>{t('Parameters')}</h4>
          <ul className='space-y-1'>
            {diff.params.map((item) => (
              <DiffRow
                key={item.key}
                title={item.key}
                onLocate={onLocate}
              >
                <div className='grid gap-1 sm:grid-cols-2'>
                  <DiffValue label={t('Before')} value={item.before} tone='bad' />
                  <DiffValue label={t('After')} value={item.after} tone='good' />
                </div>
              </DiffRow>
            ))}
          </ul>
        </section>
      )}

      {diff.system.changed && (
        <section>
          <h4 className='mb-1 font-medium'>{t('System prompt')}</h4>
          <DiffRow
            title={`${diff.system.before_chars} → ${diff.system.after_chars}`}
            onLocate={onLocate}
          >
            <div className='grid gap-1 sm:grid-cols-2'>
              <DiffValue
                label={t('Before')}
                value={diff.system.before}
                tone='bad'
                long
              />
              <DiffValue
                label={t('After')}
                value={diff.system.after}
                tone='good'
                long
              />
            </div>
          </DiffRow>
        </section>
      )}

      {(diff.messages.added.length > 0 ||
        diff.messages.removed.length > 0 ||
        diff.messages.changed.length > 0) && (
        <section>
          <h4 className='mb-1 font-medium'>
            {t('Messages ({{before}} → {{after}})', {
              before: diff.messages.before_total,
              after: diff.messages.after_total,
            })}
          </h4>
          <ul className='space-y-1'>
            {diff.messages.added.map((item) => (
              <DiffRow
                key={`a${item.index}`}
                title={`+ ${item.role}`}
                index={item.index}
                tone='good'
                onLocate={onLocate}
              >
                <p className='break-words whitespace-pre-wrap'>{item.preview}</p>
              </DiffRow>
            ))}
            {diff.messages.removed.map((item) => (
              <DiffRow
                key={`r${item.index}`}
                title={`- ${item.role}`}
                index={item.index}
                tone='bad'
                onLocate={onLocate}
              >
                <p className='break-words whitespace-pre-wrap'>{item.preview}</p>
              </DiffRow>
            ))}
            {diff.messages.changed.map((item) => (
              <DiffRow
                key={`c${item.index}`}
                title={`~ ${item.role}`}
                index={item.index}
                onLocate={onLocate}
              >
                <div className='grid gap-1'>
                  <p className='text-[11px] break-words whitespace-pre-wrap text-red-600 line-through decoration-red-400/50'>
                    {item.before}
                  </p>
                  <p className='text-[11px] break-words whitespace-pre-wrap text-emerald-700 dark:text-emerald-400'>
                    {item.after}
                  </p>
                </div>
              </DiffRow>
            ))}
          </ul>
        </section>
      )}

      {(diff.tools.added.length > 0 ||
        diff.tools.removed.length > 0 ||
        diff.tools.changed.length > 0) && (
        <section>
          <h4 className='mb-1 font-medium'>{t('Tools')}</h4>
          <div className='flex flex-wrap gap-1'>
            {diff.tools.added.map((name) => (
              <Badge
                key={`ta${name}`}
                className='bg-emerald-600/15 text-[10px] text-emerald-700 dark:text-emerald-300'
              >
                + {name}
              </Badge>
            ))}
            {diff.tools.removed.map((name) => (
              <Badge
                key={`tr${name}`}
                className='bg-red-600/15 text-[10px] text-red-600'
              >
                - {name}
              </Badge>
            ))}
            {diff.tools.changed.map((item) => (
              <button
                key={`tc${item.name}`}
                type='button'
                onClick={onLocate}
                title={t('Locate')}
                className='hover:bg-muted rounded-full border px-1.5 text-[10px]'
              >
                ~ {item.name}
              </button>
            ))}
          </div>
        </section>
      )}
    </div>
  )
}

function DiffRow({
  title,
  index,
  tone,
  onLocate,
  children,
}: {
  title: string
  index?: number
  tone?: 'good' | 'bad'
  onLocate: () => void
  children: ReactNode
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <li
      className={cn(
        'rounded-md border p-1.5',
        tone === 'good' && 'border-emerald-500/30',
        tone === 'bad' && 'border-red-500/30'
      )}
    >
      <div className='flex items-center gap-2'>
        <button
          type='button'
          onClick={() => setOpen((prev) => !prev)}
          className='flex min-w-0 flex-1 items-center gap-1 text-left'
        >
          <span className='text-muted-foreground text-[10px]'>
            {open ? '▾' : '▸'}
          </span>
          <span className='truncate font-mono text-[11px]'>{title}</span>
          {index !== undefined && (
            <span className='text-muted-foreground shrink-0 text-[10px]'>
              #{index}
            </span>
          )}
        </button>
        <button
          type='button'
          onClick={onLocate}
          className='text-muted-foreground hover:bg-muted shrink-0 rounded-full border px-1.5 text-[10px]'
        >
          {t('Locate')}
        </button>
      </div>
      {open && <div className='mt-1 pl-4 text-[11px]'>{children}</div>}
    </li>
  )
}

function DiffValue({
  label,
  value,
  tone,
  long,
}: {
  label: string
  value?: string
  tone: 'good' | 'bad'
  long?: boolean
}) {
  return (
    <div className='bg-muted/40 min-w-0 rounded p-1'>
      <span className='text-muted-foreground block text-[10px]'>{label}</span>
      <span
        className={cn(
          'block font-mono break-words',
          long && 'max-h-32 overflow-auto whitespace-pre-wrap',
          tone === 'bad'
            ? 'text-red-600'
            : 'text-emerald-700 dark:text-emerald-400'
        )}
      >
        {value ?? '—'}
      </span>
    </div>
  )
}

function Block({ title, text }: { title: string; text: string }) {
  return (
    <section>
      <div className='mb-1 flex items-center gap-2'>
        <h4 className='text-xs font-medium'>{title}</h4>
        <CopyButton value={text} size='icon' className='size-6' />
      </div>
      <pre className='bg-muted/40 max-h-96 overflow-auto rounded-md p-2 text-[11px] break-words whitespace-pre-wrap'>
        {text}
      </pre>
    </section>
  )
}

function ToolCalls({ calls }: { calls: NonNullable<TapRecon['tool_calls']> }) {
  const { t } = useTranslation()
  return (
    <section>
      <h4 className='mb-1 text-xs font-medium'>{t('Tool calls')}</h4>
      <div className='space-y-1.5'>
        {calls.map((call, index) => (
          <div
            key={call.id || `call-${index}`}
            className='bg-muted/30 rounded-md border p-2'
          >
            <div className='flex items-center justify-between gap-2'>
              <span className='truncate font-mono text-[11px]'>
                {call.name || `#${index + 1}`}
              </span>
              <CopyButton
                value={call.arguments}
                size='icon'
                className='size-6'
              />
            </div>
            <pre className='mt-1 max-h-48 overflow-auto text-[11px] break-words whitespace-pre-wrap'>
              {call.arguments || '{}'}
            </pre>
          </div>
        ))}
      </div>
    </section>
  )
}
