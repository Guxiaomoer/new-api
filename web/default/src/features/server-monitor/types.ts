export interface ApiResponse<T> {
  success: boolean
  message: string
  data: T
}

export interface ServerMonitorUsage {
  total: number
  used: number
  available: number
  used_percent: number
}

export interface ServerMonitorLoad {
  one_minute: number
  five_minutes: number
  fifteen_minutes: number
}

export interface ServerMonitorHost {
  uptime_seconds: number
  cpu_cores: number
  cpu_usage_percent: number
  load_average: ServerMonitorLoad
  memory: ServerMonitorUsage
  swap: ServerMonitorUsage
  root_disk: ServerMonitorUsage
}

export interface ServerMonitorApp {
  go_version: string
  goroutines: number
  heap_alloc: number
  heap_sys: number
  sys: number
  num_gc: number
}

export interface ServerMonitorDatabase {
  total_users: number
  enabled_users: number
  oauth_bindings: number
  total_tokens: number
  enabled_tokens: number
  recent_users_24h: number
  recent_logins_24h: number
  recent_logins_7d: number
}

export interface ServerMonitorCapacity {
  level: 'ok' | 'warning' | 'critical'
  conservative_concurrent_range: string
  registered_users_suggestion: string
  hints: string[]
}

export interface ServerMonitorOverview {
  collected_at: number
  host: ServerMonitorHost
  app: ServerMonitorApp
  database: ServerMonitorDatabase
  capacity: ServerMonitorCapacity
  warnings: string[]
  partial: boolean
}
