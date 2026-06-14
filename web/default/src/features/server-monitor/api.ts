import { api } from '@/lib/api'
import type { ApiResponse, ServerMonitorOverview } from './types'

export async function getServerMonitorOverview() {
  const res = await api.get<ApiResponse<ServerMonitorOverview>>(
    '/api/server-monitor/overview'
  )
  return res.data
}
