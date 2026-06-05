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
import { Copy, Check, AlertTriangle, Shield } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { StatusBadge } from '@/components/status-badge'
import type { InterceptLog } from '../types'

function DetailRow(props: {
  label: React.ReactNode
  value: React.ReactNode
  mono?: boolean
}) {
  return (
    <div className='grid min-w-0 grid-cols-[5.25rem_minmax(0,1fr)] gap-2 text-sm sm:grid-cols-[7rem_minmax(0,1fr)] sm:gap-3'>
      <span className='text-muted-foreground min-w-0 text-xs'>
        {props.label}
      </span>
      <span
        className={cn(
          'max-w-full min-w-0 text-xs break-all sm:break-words',
          props.mono && 'font-mono'
        )}
      >
        {props.value}
      </span>
    </div>
  )
}

function DetailSection(props: {
  icon?: React.ReactNode
  label: string
  variant?: 'default' | 'danger'
  children: React.ReactNode
}) {
  const isDanger = props.variant === 'danger'
  return (
    <div className='min-w-0 space-y-1.5'>
      <Label
        className={cn(
          'flex items-center gap-1.5 text-xs font-semibold',
          isDanger && 'text-red-500'
        )}
      >
        {props.icon}
        {props.label}
      </Label>
      <div
        className={cn(
          'min-w-0 space-y-1 overflow-hidden rounded-md border p-2.5 max-sm:p-2',
          isDanger
            ? 'border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/20'
            : 'bg-muted/30'
        )}
      >
        {props.children}
      </div>
    </div>
  )
}

/**
 * Render body content as plain text only.
 * No HTML/Markdown rendering, no auto-linked URLs.
 */
function PlainBodyBlock(props: {
  label: string
  content: string
  copyToClipboard: (text: string) => void
  copiedText: string | null
}) {
  const { label, content, copyToClipboard, copiedText } = props
  if (!content) return null
  return (
    <div className='space-y-1.5'>
      <Label className='text-xs font-semibold'>{label}</Label>
      <div className='bg-muted/30 relative min-w-0 overflow-hidden rounded-md border p-2.5'>
        <Button
          variant='ghost'
          size='sm'
          className='absolute top-1.5 right-1.5 h-5 w-5 p-0'
          onClick={() => copyToClipboard(content)}
          title='Copy to clipboard'
          aria-label='Copy to clipboard'
        >
          {copiedText === content ? (
            <Check className='size-3 text-green-600' />
          ) : (
            <Copy className='size-3' />
          )}
        </Button>
        <pre className='min-w-0 pr-6 font-mono text-[11px] leading-relaxed break-all whitespace-pre-wrap sm:break-words'>
          {content}
        </pre>
      </div>
    </div>
  )
}

const SEVERITY_VARIANT: Record<string, string> = {
  critical: 'red',
  high: 'orange',
  medium: 'yellow',
  low: 'neutral',
}

interface InterceptLogDetailDialogProps {
  log: InterceptLog | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function InterceptLogDetailDialog({
  log,
  open,
  onOpenChange,
}: InterceptLogDetailDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })

  if (!log) return null

  const severityVariant = SEVERITY_VARIANT[log.severity] ?? 'neutral'

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn(
          'min-w-0 overflow-hidden',
          'max-sm:max-h-[calc(100dvh-1.5rem)] max-sm:w-[calc(100vw-1.5rem)] max-sm:max-w-[calc(100vw-1.5rem)] max-sm:p-4',
          'sm:max-w-2xl'
        )}
      >
        <DialogHeader className='max-sm:gap-1'>
          <DialogTitle className='flex items-center gap-2 text-base'>
            {t('Intercept Log Detail')}
            {log.intercept_type && (
              <StatusBadge
                label={log.intercept_type}
                variant='neutral'
                size='sm'
                copyable={false}
              />
            )}
            {log.severity && (
              <StatusBadge
                label={log.severity}
                variant={severityVariant as 'red' | 'orange' | 'yellow' | 'neutral'}
                size='sm'
                copyable={false}
              />
            )}
          </DialogTitle>
          <DialogDescription className='sr-only'>
            {t('View the complete details for this intercept log entry')}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='max-h-[70vh] min-w-0 overflow-hidden pr-2 max-sm:max-h-[calc(100dvh-7rem)] sm:pr-4'>
          <div className='w-full max-w-full min-w-0 space-y-2.5 overflow-hidden py-1 sm:space-y-3'>
            {/* Overview */}
            <div className='min-w-0 space-y-1'>
              {log.request_id && (
                <DetailRow
                  label={t('Request ID')}
                  value={log.request_id}
                  mono
                />
              )}
              <DetailRow
                label={t('Time')}
                value={
                  log.created_at
                    ? new Date(log.created_at * 1000).toLocaleString()
                    : '-'
                }
              />
              {log.model_name && (
                <DetailRow
                  label={t('Model')}
                  value={log.model_name}
                  mono
                />
              )}
              {log.request_path && (
                <DetailRow
                  label={t('Request Path')}
                  value={log.request_path}
                  mono
                />
              )}
              <DetailRow
                label={t('Stream')}
                value={log.is_stream ? t('Yes') : t('No')}
              />
            </div>

            {/* Intercept info */}
            <DetailSection
              icon={<AlertTriangle className='size-3.5' aria-hidden='true' />}
              label={t('Intercept Info')}
              variant='danger'
            >
              {log.intercept_type && (
                <DetailRow
                  label={t('Type')}
                  value={log.intercept_type}
                  mono
                />
              )}
              {log.rule && (
                <DetailRow label={t('Rule')} value={log.rule} mono />
              )}
              {log.keyword && (
                <DetailRow label={t('Keyword')} value={log.keyword} mono />
              )}
              {log.reason && (
                <DetailRow label={t('Reason')} value={log.reason} />
              )}
              <DetailRow
                label={t('Auto-disabled')}
                value={
                  log.auto_disabled_channel ? t('Yes') : t('No')
                }
              />
            </DetailSection>

            {/* Channel info */}
            {(log.channel_id > 0 || log.channel_type > 0) && (
              <DetailSection
                icon={<Shield className='size-3.5' aria-hidden='true' />}
                label={t('Channel Info')}
              >
                {log.channel_id > 0 && (
                  <DetailRow
                    label={t('Channel ID')}
                    value={String(log.channel_id)}
                    mono
                  />
                )}
                {log.channel_type > 0 && (
                  <DetailRow
                    label={t('Channel Type')}
                    value={String(log.channel_type)}
                    mono
                  />
                )}
                {log.upstream_status_code > 0 && (
                  <DetailRow
                    label={t('Status Code')}
                    value={String(log.upstream_status_code)}
                    mono
                  />
                )}
                {log.upstream_content_type && (
                  <DetailRow
                    label={t('Content Type')}
                    value={log.upstream_content_type}
                    mono
                  />
                )}
              </DetailSection>
            )}

            {/* User info */}
            {(log.user_id > 0 || log.token_id > 0) && (
              <div className='min-w-0 space-y-1'>
                {log.user_id > 0 && (
                  <DetailRow
                    label={t('User ID')}
                    value={String(log.user_id)}
                    mono
                  />
                )}
                {log.token_id > 0 && (
                  <DetailRow
                    label={t('Token ID')}
                    value={String(log.token_id)}
                    mono
                  />
                )}
              </div>
            )}

            {/* Full bodies - plaintext only, no rendering */}
            <PlainBodyBlock
              label={t('Client Request Body')}
              content={log.full_client_request_body}
              copyToClipboard={copyToClipboard}
              copiedText={copiedText}
            />

            <PlainBodyBlock
              label={t('Upstream Response Body')}
              content={log.full_upstream_response_body}
              copyToClipboard={copyToClipboard}
              copiedText={copiedText}
            />

            <PlainBodyBlock
              label={t('Safe Response Body')}
              content={log.full_safe_response_body}
              copyToClipboard={copyToClipboard}
              copiedText={copiedText}
            />

            {/* Excerpts */}
            {log.excerpt_client_request && (
              <PlainBodyBlock
                label={t('Client Request Excerpt')}
                content={log.excerpt_client_request}
                copyToClipboard={copyToClipboard}
                copiedText={copiedText}
              />
            )}

            {log.excerpt_upstream_response && (
              <PlainBodyBlock
                label={t('Upstream Response Excerpt')}
                content={log.excerpt_upstream_response}
                copyToClipboard={copyToClipboard}
                copiedText={copiedText}
              />
            )}

            {log.excerpt_safe_response && (
              <PlainBodyBlock
                label={t('Safe Response Excerpt')}
                content={log.excerpt_safe_response}
                copyToClipboard={copyToClipboard}
                copiedText={copiedText}
              />
            )}

            {/* Metadata */}
            {log.metadata && (
              <PlainBodyBlock
                label={t('Metadata')}
                content={log.metadata}
                copyToClipboard={copyToClipboard}
                copiedText={copiedText}
              />
            )}
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
