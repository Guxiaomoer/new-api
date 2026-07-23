import { api } from '@/lib/api'
import type {
  ApiResponse,
  CommunityMonitorConfig,
  CommunityMonitorResult,
  CommunityMonitorStatus,
} from './types'

export async function getCommunityMonitorStatus() {
  const res = await api.get<ApiResponse<CommunityMonitorStatus>>(
    '/api/community_monitor/status'
  )
  return res.data
}

export async function saveCommunityMonitorConfig(config: CommunityMonitorConfig) {
  const res = await api.put<ApiResponse<CommunityMonitorConfig>>(
    '/api/community_monitor/config',
    config
  )
  return res.data
}

export async function getCommunityMonitorResults() {
  const res = await api.get<ApiResponse<CommunityMonitorResult[]>>(
    '/api/community_monitor/results'
  )
  return res.data
}

export async function scanCommunityMonitor() {
  const res = await api.post<ApiResponse<CommunityMonitorStatus>>(
    '/api/community_monitor/scan'
  )
  return res.data
}

export async function detectCommunityMonitor() {
  const res = await api.post<ApiResponse<CommunityMonitorStatus>>(
    '/api/community_monitor/detect'
  )
  return res.data
}

export async function startCommunityMonitorCollector() {
  const res = await api.post<ApiResponse<CommunityMonitorStatus>>(
    '/api/community_monitor/collector/start'
  )
  return res.data
}

export async function stopCommunityMonitorCollector() {
  const res = await api.post<ApiResponse<CommunityMonitorStatus>>(
    '/api/community_monitor/collector/stop'
  )
  return res.data
}
