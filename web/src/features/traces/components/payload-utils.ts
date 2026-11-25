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

export function prettyJson(raw: string): string {
  if (!raw) return ''
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

export function parseJson<T>(raw: string): T | null {
  if (!raw) return null
  try {
    return JSON.parse(raw) as T
  } catch {
    return null
  }
}

export function contentToText(content: unknown): string {
  if (typeof content === 'string') return content
  if (Array.isArray(content)) {
    return content
      .map((block) => {
        if (typeof block === 'string') return block
        if (block && typeof block === 'object') {
          const record = block as Record<string, unknown>
          if (typeof record.text === 'string') return record.text
          if (typeof record.content === 'string') return record.content
          if (record.type === 'tool_use' || record.type === 'tool_result') {
            return `[${String(record.type)}] ${String(record.name ?? record.tool_use_id ?? '')}`
          }
        }
        return ''
      })
      .filter(Boolean)
      .join('\n')
  }
  if (content == null) return ''
  return JSON.stringify(content)
}

export type MessageLike = { role?: unknown; content?: unknown }

export function messagesOf(body: unknown): MessageLike[] {
  if (!body || typeof body !== 'object') return []
  const record = body as Record<string, unknown>
  const list = record.messages ?? record.input
  if (!Array.isArray(list)) return []
  return list.filter((item): item is MessageLike =>
    Boolean(item) && typeof item === 'object'
  )
}

export function stableKey(text: string): string {
  return `${text.length}-${text.slice(0, 32)}`
}
