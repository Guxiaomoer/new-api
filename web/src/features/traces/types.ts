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

export type TapConfig = {
  enabled: boolean
  max_body: number
  retention_days: number
  max_sessions: number
}

export type TapStats = {
  sessions: number
  turns: number
  prompt_tokens: number
  completion_tokens: number
}

export type TapSession = {
  id: number
  fingerprint: string
  key_id: number
  key_name: string
  client: string
  client_ua: string
  first_user: string
  kind: string
  parent_id: number
  protocol: string
  model: string
  started_at: number
  updated_at: number
  turns: number
  prompt_tokens: number
  completion_tokens: number
}

export type TapSessionListItem = TapSession & {
  actual_turns: number
  aux_count: number
}

export type TapTurn = {
  id: number
  session_id: number
  seq: number
  ts: number
  path: string
  protocol: string
  model: string
  mapped_model: string
  status: number
  stream: boolean
  latency_ms: number
  first_token_ms: number
  prompt_tokens: number
  completion_tokens: number
  cache_hit_tokens: number
  cache_miss_tokens: number
  cost: number
  trace_id: string
  error: string
  truncated: boolean
}

export type TapTurnDetail = TapTurn & {
  request_json: string
  request_digest: string
  response_text: string
  client_response_text: string
  recon_json: string
  usage_json: string
}

export type TapToolCall = {
  id?: string
  name: string
  arguments: string
}

export type TapRecon = {
  text: string
  reasoning: string
  tool_calls: TapToolCall[] | null
}

export type TapDescribe = {
  kind: string
  messages: number
  roles: Record<string, number>
  tools: number
  tool_calls: number
  system_chars: number
}

export type TapDiffMessage = {
  index: number
  role: string
  preview?: string
  before?: string
  after?: string
  chars?: number
}

export type TapDiff = {
  before: { id: number; seq: number; ts: number; model: string }
  after: { id: number; seq: number; ts: number; model: string }
  params: { key: string; before: string; after: string }[]
  system: {
    changed: boolean
    before_chars: number
    after_chars: number
    before: string
    after: string
  }
  messages: {
    before_total: number
    after_total: number
    common: number
    added: TapDiffMessage[]
    removed: TapDiffMessage[]
    changed: TapDiffMessage[]
  }
  tools: {
    added: string[]
    removed: string[]
    changed: { name: string; before: string; after: string }[]
  }
  summary: {
    params: number
    system: number
    added: number
    removed: number
    changed: number
    tools: number
  }
  from_digest?: boolean
}

export type ApiResult<T> = {
  success: boolean
  message?: string
  data?: T
}
