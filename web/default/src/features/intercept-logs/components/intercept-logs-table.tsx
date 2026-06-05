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
import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  flexRender,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getPaginationRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useMediaQuery } from '@/hooks'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { TableCell, TableRow } from '@/components/ui/table'
import { DataTablePage } from '@/components/data-table'
import type { InterceptLog } from '../types'
import { getInterceptLogs } from '../api'
import { useInterceptLogsColumns } from './columns'
import { InterceptLogsFilterBar } from './intercept-logs-filter-bar'

const route = getRouteApi('/_authenticated/intercept-logs/$section')

const DEFAULT_DATA = {
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
}

export function InterceptLogsTable() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const searchParams = route.useSearch()

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 20 : 100 },
    globalFilter: { enabled: false },
    columnFilters: [
      { columnId: 'model_name', searchKey: 'model', type: 'string' as const },
      { columnId: 'channel_id', searchKey: 'channel', type: 'string' as const },
      {
        columnId: 'intercept_type',
        searchKey: 'interceptType',
        type: 'string' as const,
      },
      { columnId: 'keyword', searchKey: 'keyword', type: 'string' as const },
      { columnId: 'severity', searchKey: 'severity', type: 'string' as const },
      {
        columnId: 'request_id',
        searchKey: 'requestId',
        type: 'string' as const,
      },
    ],
  })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'intercept-logs',
      pagination.pageIndex + 1,
      pagination.pageSize,
      columnFilters,
    ],
    queryFn: async () => {
      const params: Record<string, unknown> = {
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      }

      // Map search params to API params
      if (searchParams.model) params.model_name = String(searchParams.model)
      if (searchParams.channel) params.channel_id = Number(searchParams.channel)
      if (searchParams.interceptType)
        params.intercept_type = String(searchParams.interceptType)
      if (searchParams.keyword) params.keyword = String(searchParams.keyword)
      if (searchParams.severity) params.severity = String(searchParams.severity)
      if (searchParams.requestId)
        params.request_id = String(searchParams.requestId)

      // Time range
      if (searchParams.startTime) {
        params.start_timestamp = Math.floor(
          (searchParams.startTime as number) / 1000
        )
      }
      if (searchParams.endTime) {
        params.end_timestamp = Math.floor(
          (searchParams.endTime as number) / 1000
        )
      }

      // Map column filters
      if (columnFilters.length > 0) {
        for (const { id, value } of columnFilters) {
          if (value === undefined || value === null || value === '') continue
          switch (id) {
            case 'model_name':
              params.model_name = String(value)
              break
            case 'channel_id':
              params.channel_id = Number(value)
              break
            case 'intercept_type':
              params.intercept_type = String(value)
              break
            case 'keyword':
              params.keyword = String(value)
              break
            case 'severity':
              params.severity = String(value)
              break
            case 'request_id':
              params.request_id = String(value)
              break
          }
        }
      }

      const result = await getInterceptLogs(params)

      if (!result?.success) {
        toast.error(result?.message || t('Failed to load intercept logs'))
        return DEFAULT_DATA
      }

      return result.data || DEFAULT_DATA
    },
    placeholderData: (previousData) => previousData,
  })

  const logs = data?.items || []
  const columns = useInterceptLogsColumns()
  const isLoadingData = isLoading || (isFetching && !data)

  const table = useReactTable<InterceptLog>({
    data: logs,
    columns,
    state: {
      columnFilters,
      pagination,
    },
    enableRowSelection: false,
    onPaginationChange,
    onColumnFiltersChange,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    manualPagination: true,
    manualFiltering: true,
    pageCount: Math.ceil((data?.total || 0) / pagination.pageSize),
  })

  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [pageCount, ensurePageInRange])

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoadingData}
      isFetching={isFetching}
      emptyTitle={t('No Intercept Logs')}
      emptyDescription={t(
        'No intercept logs found. Intercept logs are created when upstream responses are blocked by safety filters.'
      )}
      skeletonKeyPrefix='intercept-log-skeleton'
      tableClassName={cn(
        'overflow-x-auto',
        '[&_[data-slot=table]]:text-[13px] [&_[data-slot=table]_td]:text-[13px] [&_[data-slot=table]_td_*]:text-[13px] [&_[data-slot=table]_th]:text-[13px] [&_[data-slot=table]_th_*]:text-[13px]'
      )}
      tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
      toolbar={<InterceptLogsFilterBar table={table} />}
      renderRow={(row) => (
        <TableRow key={row.id}>
          {row.getVisibleCells().map((cell) => (
            <TableCell key={cell.id} className='py-2'>
              {flexRender(cell.column.columnDef.cell, cell.getContext())}
            </TableCell>
          ))}
        </TableRow>
      )}
    />
  )
}
