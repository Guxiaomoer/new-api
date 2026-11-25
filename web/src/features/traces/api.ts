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
import type {
  ApiResult,
  TapConfig,
  TapDescribe,
  TapDiff,
  TapRecon,
  TapSession,
  TapSessionListItem,
  TapStats,
  TapTurn,
  TapTurnDetail,
} from './types'

export type TapConfigPayload = {
  config: TapConfig
  stats: TapStats
}

export type ListTapSessionsParams = {
  p?: number
  page_size?: number
  days?: number
  client?: string
  q?: string
  include_aux?: boolean
}

export async function getTapConfig(): Promise<ApiResult<TapConfigPayload>> {
  const res = await api.get('/api/tap/config')
  return res.data
}

export async function updateTapConfig(
  config: TapConfig
): Promise<ApiResult<TapConfig>> {
  const res = await api.put('/api/tap/config', config)
  return res.data
}

export async function listTapSessions(
  params: ListTapSessionsParams
): Promise<
  ApiResult<{
    items: TapSessionListItem[]
    total: number
    page: number
    page_size: number
  }>
> {
  const res = await api.get('/api/tap/sessions', { params })
  return res.data
}

export async function getTapSession(id: number): Promise<
  ApiResult<{
    session: TapSession
    turns: TapTurn[]
    aux: TapSession[]
  }>
> {
  const res = await api.get(`/api/tap/sessions/${id}`)
  return res.data
}

export async function getTapTurn(id: number): Promise<
  ApiResult<{
    turn: TapTurnDetail
    describe: TapDescribe
    client_recon: TapRecon | null
  }>
> {
  const res = await api.get(`/api/tap/turns/${id}`)
  return res.data
}

export async function getTapDiff(
  before: number,
  after: number
): Promise<ApiResult<TapDiff>> {
  const res = await api.get('/api/tap/diff', { params: { before, after } })
  return res.data
}

export async function deleteTapSession(id: number): Promise<ApiResult<null>> {
  const res = await api.delete(`/api/tap/sessions/${id}`)
  return res.data
}

export async function pruneTap(): Promise<
  ApiResult<{ deleted: number }>
> {
  const res = await api.post('/api/tap/prune')
  return res.data
}

export async function clearTap(): Promise<ApiResult<null>> {
  const res = await api.post('/api/tap/clear')
  return res.data
}

export async function backfillTap(): Promise<
  ApiResult<{ updated: number }>
> {
  const res = await api.post('/api/tap/backfill')
  return res.data
}

export async function downloadTapExport(id: number): Promise<void> {
  const res = await api.get(`/api/tap/sessions/${id}/export`, {
    responseType: 'blob',
  })
  const url = URL.createObjectURL(res.data as Blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `tap-trace-${id}.html`
  link.click()
  URL.revokeObjectURL(url)
}
