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

export type InterceptLog = {
  id: number
  created_at: number
  request_id: string
  user_id: number
  token_id: number
  channel_id: number
  channel_type: number
  model_name: string
  request_path: string
  is_stream: boolean
  intercept_type: string
  rule: string
  reason: string
  keyword: string
  severity: string
  auto_disabled_channel: boolean
  upstream_status_code: number
  upstream_content_type: string
  content_hashes: string
  metadata: string
  full_client_request_body: string
  full_upstream_response_body: string
  full_safe_response_body: string
  excerpt_client_request: string
  excerpt_upstream_response: string
  excerpt_safe_response: string
}

export type GetInterceptLogsParams = {
  p?: number
  page_size?: number
  request_id?: string
  user_id?: number
  token_id?: number
  channel_id?: number
  channel_type?: number
  model_name?: string
  request_path?: string
  intercept_type?: string
  rule?: string
  keyword?: string
  severity?: string
  auto_disabled_channel?: boolean
  upstream_status_code?: number
  start_timestamp?: number
  end_timestamp?: number
}

export type GetInterceptLogsResponse = {
  success: boolean
  message?: string
  data?: {
    items: InterceptLog[]
    total: number
    page: number
    page_size: number
  }
}

export type GetInterceptLogDetailResponse = {
  success: boolean
  message?: string
  data?: InterceptLog
}

export type DeleteInterceptLogsResponse = {
  success: boolean
  message?: string
  data?: number
}
