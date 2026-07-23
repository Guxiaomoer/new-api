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

type ApiResponse<T> = {
  success: boolean
  message: string
  data: T
}

export type CommunitySyncResult = {
  dry_run: boolean
  member_count: number
  local_user_count: number
  restricted_count: number
  unrestricted_count: number
  protected_skipped: number
  restrict_users: string[]
  unrestrict_users: string[]
  error?: string
}

export async function previewCommunitySync() {
  const res = await api.post<ApiResponse<CommunitySyncResult>>(
    '/api/community_sync/preview'
  )
  return res.data
}

export async function runCommunitySync() {
  const res = await api.post<ApiResponse<CommunitySyncResult>>(
    '/api/community_sync/sync'
  )
  return res.data
}
