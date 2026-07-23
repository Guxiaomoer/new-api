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
import { useTranslation } from 'react-i18next'
import { type Table } from '@tanstack/react-table'
import { LogsFilterInput } from '@/features/usage-logs/components/logs-filter-toolbar'

import type { InterceptLog } from '../types'

interface InterceptLogsFilterBarProps {
  table: Table<InterceptLog>
}

export function InterceptLogsFilterBar({
  table,
}: InterceptLogsFilterBarProps) {
  const { t } = useTranslation()

  const modelFilter =
    (table.getColumn('model_name')?.getFilterValue() as string) ?? ''
  const channelFilter =
    (table.getColumn('channel_id')?.getFilterValue() as string) ?? ''
  const interceptTypeFilter =
    (table.getColumn('intercept_type')?.getFilterValue() as string) ?? ''
  const keywordFilter =
    (table.getColumn('keyword')?.getFilterValue() as string) ?? ''
  const severityFilter =
    (table.getColumn('severity')?.getFilterValue() as string) ?? ''
  const requestIdFilter =
    (table.getColumn('request_id')?.getFilterValue() as string) ?? ''

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <LogsFilterInput
        placeholder={t('Model')}
        value={modelFilter}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          table.getColumn('model_name')?.setFilterValue(e.target.value)
        }
        className='h-8 w-32'
      />
      <LogsFilterInput
        placeholder={t('Channel ID')}
        value={channelFilter}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          table.getColumn('channel_id')?.setFilterValue(e.target.value)
        }
        className='h-8 w-28'
      />
      <LogsFilterInput
        placeholder={t('Intercept Type')}
        value={interceptTypeFilter}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          table.getColumn('intercept_type')?.setFilterValue(e.target.value)
        }
        className='h-8 w-32'
      />
      <LogsFilterInput
        placeholder={t('Keyword')}
        value={keywordFilter}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          table.getColumn('keyword')?.setFilterValue(e.target.value)
        }
        className='h-8 w-32'
      />
      <LogsFilterInput
        placeholder={t('Severity')}
        value={severityFilter}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          table.getColumn('severity')?.setFilterValue(e.target.value)
        }
        className='h-8 w-28'
      />
      <LogsFilterInput
        placeholder={t('Request ID')}
        value={requestIdFilter}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          table.getColumn('request_id')?.setFilterValue(e.target.value)
        }
        className='h-8 w-40'
      />
    </div>
  )
}
