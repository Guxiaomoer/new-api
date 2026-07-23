import { api } from '@/lib/api'

type ApiResponse<T> = {
  success: boolean
  message: string
  data: T
}

export type CommunityCheckinBotStatus = {
  enabled: boolean
  room_id: string
  bot_user_id: string
  bot_name: string
  interval_seconds: number
  min_usd: number
  max_usd: number
  last_message_id: string
  authorization_set: boolean
  fingerprint_set: boolean
  last_run_at: number
  last_processed_count: number
  last_triggered_count: number
  last_rewarded_count: number
  last_error: string
}

export type CommunityCheckinBotRunResult = {
  processed_count: number
  triggered_count: number
  rewarded_count: number
  last_message_id: string
  error?: string
}

export async function getCommunityCheckinBotStatus() {
  const res = await api.get<ApiResponse<CommunityCheckinBotStatus>>(
    '/api/community_checkin_bot/status'
  )
  return res.data
}

export async function runCommunityCheckinBot() {
  const res = await api.post<ApiResponse<CommunityCheckinBotRunResult>>(
    '/api/community_checkin_bot/run'
  )
  return res.data
}
