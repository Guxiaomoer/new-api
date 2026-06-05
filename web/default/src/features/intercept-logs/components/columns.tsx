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
import { Eye } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import type { InterceptLog } from '../types'
import { getInterceptLogDetail } from '../api'
import { InterceptLogDetailDialog } from './intercept-log-detail-dialog'

const SEVERITY_VARIANT: Record<string, string> = {
  critical: 'red',
  high: 'orange',
  medium: 'yellow',
  low: 'neutral',
}

function formatTimestamp(ts: number): string {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString()
}

function TruncatedCell({ value, maxLen = 40 }: { value: string; maxLen?: number }) {
  if (!value) return <span className='text-muted-foreground'>-</span>
  const truncated = value.length > maxLen ? value.slice(0, maxLen) + '...' : value
  return (
    <span className='max-w-[200px] truncate' title={value}>
      {truncated}
    </span>
  )
}

function ActionsCell({ log }: { log: InterceptLog }) {
  const { t } = useTranslation()
  const [detailOpen, setDetailOpen] = useState(false)

  const detailQuery = useQuery({
    queryKey: ['intercept-log-detail', log.id],
    queryFn: async () => {
      const result = await getInterceptLogDetail(log.id)
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Failed to load intercept log detail'))
      }
      return result.data
    },
    enabled: detailOpen,
  })

  const handleOpenDetail = () => {
    setDetailOpen(true)
  }

  useEffect(() => {
    if (!detailQuery.error) return
    toast.error(
      detailQuery.error instanceof Error
        ? detailQuery.error.message
        : t('Failed to load intercept log detail')
    )
  }, [detailQuery.error, t])

  return (
    <>
      <Button
        variant='ghost'
        size='sm'
        className='h-7 w-7 p-0'
        onClick={handleOpenDetail}
        title={t('View Details')}
      >
        <Eye className='size-3.5' />
      </Button>
      <InterceptLogDetailDialog
        log={detailQuery.data ?? (detailQuery.isLoading ? null : log)}
        open={detailOpen}
        onOpenChange={setDetailOpen}
      />
    </>
  )
}

export function useInterceptLogsColumns(): ColumnDef<InterceptLog>[] {
  const { t } = useTranslation()

  return [
    {
      accessorKey: 'id',
      header: t('ID'),
      cell: ({ row }) => (
        <span className='font-mono text-xs'>
          {row.original.id}
        </span>
      ),
      size: 60,
    },
    {
      accessorKey: 'created_at',
      header: t('Time'),
      cell: ({ row }) => (
        <span className='text-xs whitespace-nowrap'>
          {formatTimestamp(row.original.created_at)}
        </span>
      ),
      size: 150,
    },
    {
      accessorKey: 'intercept_type',
      header: t('Type'),
      cell: ({ row }) => {
        const log = row.original
        return log.intercept_type ? (
          <StatusBadge
            label={log.intercept_type}
            variant='neutral'
            size='sm'
            copyable={false}
          />
        ) : null
      },
      size: 100,
    },
    {
      accessorKey: 'severity',
      header: t('Severity'),
      cell: ({ row }) => {
        const log = row.original
        if (!log.severity) return null
        const variant = SEVERITY_VARIANT[log.severity] ?? 'neutral'
        return (
          <StatusBadge
            label={log.severity}
            variant={variant as 'red' | 'orange' | 'yellow' | 'neutral'}
            size='sm'
            copyable={false}
          />
        )
      },
      size: 80,
    },
    {
      accessorKey: 'keyword',
      header: t('Keyword'),
      cell: ({ row }) => (
        <TruncatedCell
          value={row.original.keyword}
          maxLen={30}
        />
      ),
      size: 120,
    },
    {
      accessorKey: 'model_name',
      header: t('Model'),
      cell: ({ row }) => (
        <span className='font-mono text-xs'>
          {row.original.model_name || '-'}
        </span>
      ),
      size: 120,
    },
    {
      accessorKey: 'channel_id',
      header: t('Channel'),
      cell: ({ row }) => {
        const log = row.original
        return log.channel_id > 0 ? (
          <span className='font-mono text-xs'>#{log.channel_id}</span>
        ) : (
          <span className='text-muted-foreground'>-</span>
        )
      },
      size: 70,
    },
    {
      accessorKey: 'rule',
      header: t('Rule'),
      cell: ({ row }) => (
        <TruncatedCell
          value={row.original.rule}
          maxLen={30}
        />
      ),
      size: 100,
    },
    {
      accessorKey: 'auto_disabled_channel',
      header: t('Auto-disabled'),
      cell: ({ row }) => {
        const log = row.original
        return log.auto_disabled_channel ? (
          <StatusBadge label='Yes' variant='red' size='sm' copyable={false} />
        ) : (
          <span className='text-muted-foreground text-xs'>No</span>
        )
      },
      size: 90,
    },
    {
      accessorKey: 'upstream_status_code',
      header: t('Status'),
      cell: ({ row }) => {
        const code = row.original.upstream_status_code
        return code > 0 ? (
          <span className='font-mono text-xs'>{code}</span>
        ) : (
          <span className='text-muted-foreground'>-</span>
        )
      },
      size: 70,
    },
    {
      accessorKey: 'request_id',
      header: t('Request ID'),
      cell: ({ row }) => (
        <TruncatedCell
          value={row.original.request_id}
          maxLen={20}
        />
      ),
      size: 100,
    },
    {
      id: 'actions',
      header: '',
      cell: ({ row }) => (
        <ActionsCell log={row.original} />
      ),
      size: 50,
    },
  ]
}
