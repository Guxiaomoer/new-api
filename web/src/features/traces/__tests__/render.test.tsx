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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { Traces } from '..'

vi.mock('../api', () => ({
  getTapConfig: vi.fn(async () => ({
    success: true,
    data: {
      config: {
        enabled: true,
        max_body: 262144,
        retention_days: 7,
        max_sessions: 2000,
      },
      stats: {
        sessions: 2,
        turns: 5,
        prompt_tokens: 1200,
        completion_tokens: 800,
      },
    },
  })),
  listTapSessions: vi.fn(async () => ({
    success: true,
    data: {
      items: [
        {
          id: 1,
          fingerprint: 'abcdef123456',
          key_id: 1,
          key_name: 'default',
          client: 'claude',
          client_ua: 'claude-cli/1.0.0',
          first_user: 'hello',
          kind: 'main',
          parent_id: 0,
          protocol: 'claude',
          model: 'claude-sonnet-5',
          started_at: 1764000000,
          updated_at: 1764000100,
          turns: 3,
          prompt_tokens: 900,
          completion_tokens: 600,
          actual_turns: 3,
          aux_count: 1,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    },
  })),
  getTapSession: vi.fn(async () => ({
    success: true,
    data: {
      session: {
        id: 1,
        fingerprint: 'abcdef123456',
        key_id: 1,
        key_name: 'default',
        client: 'claude',
        client_ua: 'claude-cli/1.0.0',
        first_user: 'hello',
        kind: 'main',
        parent_id: 0,
        protocol: 'claude',
        model: 'claude-sonnet-5',
        started_at: 1764000000,
        updated_at: 1764000100,
        turns: 1,
        prompt_tokens: 900,
        completion_tokens: 600,
      },
      turns: [
        {
          id: 10,
          session_id: 1,
          seq: 1,
          ts: 1764000000,
          path: '/v1/messages',
          protocol: 'claude',
          model: 'claude-sonnet-5',
          mapped_model: 'claude-sonnet-5',
          status: 200,
          stream: true,
          latency_ms: 1500,
          first_token_ms: 300,
          prompt_tokens: 900,
          completion_tokens: 600,
          cache_hit_tokens: 0,
          cache_miss_tokens: 900,
          cost: 0.01,
          trace_id: 't1',
          error: '',
          truncated: false,
        },
      ],
      aux: [],
    },
  })),
  getTapTurn: vi.fn(async () => ({
    success: true,
    data: {
      turn: {
        id: 10,
        session_id: 1,
        seq: 1,
        ts: 1764000000,
        path: '/v1/messages',
        protocol: 'claude',
        model: 'claude-sonnet-5',
        mapped_model: 'claude-sonnet-5',
        status: 200,
        stream: true,
        latency_ms: 1500,
        first_token_ms: 300,
        prompt_tokens: 900,
        completion_tokens: 600,
        cache_hit_tokens: 0,
        cache_miss_tokens: 900,
        cost: 0.01,
        trace_id: 't1',
        error: '',
        truncated: false,
        request_json: JSON.stringify({
          headers: { 'content-type': 'application/json' },
          body: { model: 'claude-sonnet-5', messages: [] },
          upstream_body: { model: 'claude-sonnet-5', messages: [] },
        }),
        request_digest: '{}',
        response_text: 'data: {}\n\n',
        client_response_text: '',
        recon_json: JSON.stringify({ text: 'hi', reasoning: '', tool_calls: null }),
        usage_json: '{}',
      },
      describe: {
        kind: 'main',
        messages: 1,
        roles: { user: 1 },
        tools: 0,
        tool_calls: 0,
        tool_names: [],
        system_chars: 10,
      },
      client_recon: null,
    },
  })),
  getTapDiff: vi.fn(async () => ({ success: true, data: null })),
  updateTapConfig: vi.fn(async (next) => ({ success: true, data: next })),
  pruneTap: vi.fn(async () => ({ success: true, data: { deleted: 0 } })),
  clearTap: vi.fn(async () => ({ success: true, data: null })),
  deleteTapSession: vi.fn(async () => ({ success: true, data: null })),
  downloadTapExport: vi.fn(async () => undefined),
}))

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <Traces />
    </QueryClientProvider>
  )
}

test('renders the workspace with refresh controls and sessions', async () => {
  renderPage()

  expect(screen.getByText('Call Traces')).toBeInTheDocument()
  expect(screen.getByText('Refresh')).toBeInTheDocument()
  expect(screen.getByText('Auto refresh')).toBeInTheDocument()

  await waitFor(() => {
    expect(screen.getByText(/3 turns/)).toBeInTheDocument()
  })
  expect(screen.getByText('Pick a session on the left')).toBeInTheDocument()
})
