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
import { toIntlLocale } from '@/i18n/languages'
import { formatCompactNumber, formatTimestampRelative } from '@/lib/format'
import type { TapRecon, TapTurnDetail } from './types'

const CLIENT_LABELS: Record<string, string> = {
  claude: 'Claude Code',
  codex: 'Codex',
  dsh: 'DeepSeek Harness',
  cursor: 'Cursor',
  vscode: 'VS Code',
  cli: 'CLI',
}

export function clientLabel(client: string, fallback: string): string {
  if (!client) return fallback
  return CLIENT_LABELS[client] ?? client
}

const VERSION_RE = /([A-Za-z][A-Za-z0-9_.-]*)\/(\d+(?:\.\d+)+)/

export function clientVersion(ua: string): string {
  const match = VERSION_RE.exec(ua || '')
  return match ? match[2] : ''
}

export function formatDuration(ms: number): string {
  if (!ms || ms <= 0) return '—'
  return ms < 1000 ? `${Math.round(ms)}ms` : `${(ms / 1000).toFixed(1)}s`
}

export function formatSize(bytes: number): string {
  const value = Number(bytes || 0)
  if (!Number.isFinite(value) || value <= 0) return '0 B'
  if (value < 1024) return `${value} B`
  if (value < 1048576) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1048576).toFixed(2)} MB`
}

// i18next 的语言码是 zhCN/zhTW 这类非标准写法，Intl 会直接抛 RangeError。
export function relativeTime(ts: number, locale: string): string {
  return formatTimestampRelative(ts, 'seconds', toIntlLocale(locale))
}

export function tokenPair(prompt: number, completion: number): string {
  return `${formatCompactNumber(prompt)} / ${formatCompactNumber(completion)}`
}

export function prettyJson(value: unknown): string {
  if (value === null || value === undefined) return '—'
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

export function asObject(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null
}

export function parseJson<T>(text: string): T | null {
  if (!text) return null
  try {
    return JSON.parse(text) as T
  } catch {
    return null
  }
}

export type RequestDoc = {
  headers?: Record<string, string>
  body?: unknown
  upstream_headers?: Record<string, string>
  upstream_body?: unknown
}

export function requestDoc(turn: TapTurnDetail): RequestDoc | null {
  return parseJson<RequestDoc>(turn.request_json)
}

export function reconOf(turn: TapTurnDetail): TapRecon | null {
  const recon = parseJson<TapRecon>(turn.recon_json)
  return recon && (recon.text || recon.reasoning || recon.tool_calls?.length)
    ? recon
    : null
}

// 请求体被截断后是非法 JSON，解析必然失败；这里区分「本来就没有」和「解析不了」。
export function bodyOf(doc: RequestDoc | null, key: 'body' | 'upstream_body') {
  if (!doc) return undefined
  const value = doc[key]
  return value ?? undefined
}

export function headerPairs(
  headers: Record<string, string> | undefined | null
): Array<[string, string]> {
  if (!headers) return []
  const seen = new Set<string>()
  const pairs: Array<[string, string]> = []
  for (const [name, value] of Object.entries(headers)) {
    const lower = name.toLowerCase()
    if (seen.has(lower)) continue
    seen.add(lower)
    pairs.push([name, typeof value === 'string' ? value : prettyJson(value)])
  }
  return pairs.sort((a, b) => a[0].toLowerCase().localeCompare(b[0].toLowerCase()))
}

// 视角切到「网关」时最能说明问题的，是网关替客户端改了哪些请求字段。
export function rewrittenKeys(
  client: Record<string, unknown> | null,
  upstream: Record<string, unknown> | null
): string[] {
  if (!client || !upstream) return []
  const keys = new Set([...Object.keys(client), ...Object.keys(upstream)])
  const changed: string[] = []
  for (const key of keys) {
    if (JSON.stringify(client[key]) !== JSON.stringify(upstream[key])) {
      changed.push(key)
    }
  }
  return changed.sort()
}
