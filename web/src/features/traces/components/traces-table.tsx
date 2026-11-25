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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type PaginationState,
} from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { Search } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select'
import { TableCell, TableRow } from '@/components/ui/table'
import { DataTablePage } from '@/components/data-table'
import { formatCompactNumber, formatTimestamp } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { listTapSessions } from '../api'
import type { TapSessionListItem } from '../types'

const DAYS_OPTIONS = [1, 7, 30, 90]

export function TracesTable() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 20,
  })
  const [days, setDays] = useState(7)
  const [client, setClient] = useState('')
  const [search, setSearch] = useState('')
  const [includeAux, setIncludeAux] = useState(false)

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'tap-sessions',
      pagination.pageIndex,
      pagination.pageSize,
      days,
      client,
      search,
      includeAux,
    ],
    queryFn: async () => {
      const result = requireServerSuccess(
        await listTapSessions({
          p: pagination.pageIndex + 1,
          page_size: pagination.pageSize,
          days,
          client: client || undefined,
          q: search || undefined,
          include_aux: includeAux || undefined,
        })
      )
      if (!result.data) throw new Error(t('Failed to load call traces'))
      return result.data
    },
    placeholderData: (previous) => previous,
  })

  const columns = useMemo<ColumnDef<TapSessionListItem>[]>(
    () => [
      {
        accessorKey: 'client',
        header: t('Client'),
        cell: ({ row }) => (
          <div className='flex items-center gap-1.5'>
            <span>{row.original.client || 'unknown'}</span>
            {row.original.kind !== 'main' && (
              <Badge variant='secondary'>{row.original.kind}</Badge>
            )}
          </div>
        ),
      },
      {
        accessorKey: 'model',
        header: t('Model'),
        cell: ({ row }) => row.original.model || '-',
      },
      {
        accessorKey: 'actual_turns',
        header: t('Turns'),
        cell: ({ row }) => (
          <div className='flex items-center gap-1.5'>
            <span>{row.original.actual_turns}</span>
            {row.original.aux_count > 0 && (
              <Badge variant='outline'>
                +{row.original.aux_count} {t('aux')}
              </Badge>
            )}
          </div>
        ),
      },
      {
        accessorKey: 'prompt_tokens',
        header: t('Tokens'),
        cell: ({ row }) =>
          formatCompactNumber(
            row.original.prompt_tokens + row.original.completion_tokens
          ),
      },
      {
        accessorKey: 'first_user',
        header: t('Opening message'),
        cell: ({ row }) => (
          <span className='block max-w-[320px] truncate' title={row.original.first_user}>
            {row.original.first_user || '-'}
          </span>
        ),
      },
      {
        accessorKey: 'key_name',
        header: t('Key'),
        cell: ({ row }) => row.original.key_name || '-',
      },
      {
        accessorKey: 'started_at',
        header: t('Started'),
        cell: ({ row }) => (
          <span className='whitespace-nowrap'>
            {formatTimestamp(row.original.started_at)}
          </span>
        ),
      },
      {
        accessorKey: 'updated_at',
        header: t('Updated'),
        cell: ({ row }) => (
          <span className='whitespace-nowrap'>
            {formatTimestamp(row.original.updated_at)}
          </span>
        ),
      },
    ],
    [t]
  )

  const table = useReactTable({
    data: data?.items ?? [],
    columns,
    state: { pagination },
    onPaginationChange: setPagination,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: Math.ceil((data?.total ?? 0) / pagination.pageSize),
  })

  const openSession = (id: number) =>
    navigate({ to: '/traces/$id', params: { id: String(id) } })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No call traces yet')}
      emptyDescription={t(
        'Turn on recording and send one request; the client payload, upstream payload and both responses will show up here.'
      )}
      skeletonKeyPrefix='tap-session-skeleton'
      renderRow={(row) => (
        <TableRow
          key={row.id}
          className='cursor-pointer'
          onClick={() => openSession(row.original.id)}
        >
          {row.getVisibleCells().map((cell) => (
            <TableCell key={cell.id} className='py-2'>
              {flexRender(cell.column.columnDef.cell, cell.getContext())}
            </TableCell>
          ))}
        </TableRow>
      )}
      toolbar={
        <div className='flex flex-wrap items-center gap-2'>
          <div className='relative'>
            <Search className='text-muted-foreground absolute top-1/2 left-2 size-4 -translate-y-1/2' />
            <Input
              className='h-8 w-56 pl-8'
              placeholder={t('Search model, fingerprint or message')}
              value={search}
              onChange={(e) => {
                setSearch(e.target.value)
                setPagination((prev) => ({ ...prev, pageIndex: 0 }))
              }}
            />
          </div>
          <Input
            className='h-8 w-40'
            placeholder={t('Client')}
            value={client}
            onChange={(e) => {
              setClient(e.target.value)
              setPagination((prev) => ({ ...prev, pageIndex: 0 }))
            }}
          />
          <NativeSelect
            value={String(days)}
            onChange={(e) => {
              setDays(Number(e.target.value))
              setPagination((prev) => ({ ...prev, pageIndex: 0 }))
            }}
          >
            {DAYS_OPTIONS.map((option) => (
              <NativeSelectOption key={option} value={String(option)}>
                {t('Last {{days}} days', { days: option })}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <label className='text-muted-foreground flex items-center gap-1.5 text-xs'>
            <Checkbox
              checked={includeAux}
              onCheckedChange={(checked) => {
                setIncludeAux(checked === true)
                setPagination((prev) => ({ ...prev, pageIndex: 0 }))
              }}
            />
            {t('Include aux calls')}
          </label>
        </div>
      }
    />
  )
}
