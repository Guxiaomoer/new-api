export interface ApiResponse<T> {
  success: boolean
  message: string
  data: T
}

export interface CommunityMonitorConfig {
  source_url: string
  room_id: string
  user_id: string
  room_url: string
  start_time: string
  end_time: string
  query: string
  extract_regex: string
  scan_limit: number
  page_size: number
  collector_interval_minutes: number
  access_token?: string
  access_token_configured?: boolean
  headers: Record<string, string>
  detection_base_url: string
  config_path?: string
  state_path?: string
  api_type?: string
}

export interface CommunityMonitorProgress {
  checked: number
  read: number
  pages: number
  hits: number
  duplicates: number
  percent: number
  scan_started_at: string
  scan_ended_at: string
}

export interface CommunityMonitorResult {
  fingerprint: string
  masked_value: string
  kind: string
  status: string
  reason: string
  source: string
  detected_at: string
  created_at: string
}

export interface CommunityMonitorState {
  progress: CommunityMonitorProgress
  results: CommunityMonitorResult[]
  message_count: number
  candidate_count: number
  detected_count: number
  valid_count: number
  failure_cache: number
  last_run_at: string
  next_run_at: string
  last_error: string
  collector_running: boolean
  last_message_id?: string
}

export interface CommunityMonitorRule {
  name: string
  query: string
  regex: string
}

export interface CommunityMonitorStatus {
  config: CommunityMonitorConfig
  state: CommunityMonitorState
  rules: CommunityMonitorRule[]
  running: boolean
}
