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
import { api } from '@/lib/api'
import { buildQueryParams } from '../usage-logs/lib/utils'
import type {
  DeleteInterceptLogsResponse,
  GetInterceptLogDetailResponse,
  GetInterceptLogsParams,
  GetInterceptLogsResponse,
} from './types'

export async function getInterceptLogs(
  params: GetInterceptLogsParams = {}
): Promise<GetInterceptLogsResponse> {
  const queryParams = buildQueryParams({
    p: params.p || 1,
    page_size: params.page_size || 20,
    ...params,
  })
  const res = await api.get(`/api/intercept_log?${queryParams}`)
  return res.data
}

export async function getInterceptLogDetail(
  id: number
): Promise<GetInterceptLogDetailResponse> {
  const res = await api.get(`/api/intercept_log/${id}`)
  return res.data
}

export async function deleteInterceptLogs(
  targetTimestamp: number
): Promise<DeleteInterceptLogsResponse> {
  const res = await api.delete('/api/intercept_log', {
    params: { target_timestamp: targetTimestamp },
  })
  return res.data
}
