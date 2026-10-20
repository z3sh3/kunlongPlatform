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
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  ApiResponse,
  DifyAgentAccess,
  DifyAgentListParams,
  DifyAgentsPage,
} from './types'

const BASE_URL = '/api/agent-workshop'

function buildQuery(params: DifyAgentListParams): string {
  const query = new URLSearchParams()
  query.set('p', String(params.p ?? 1))
  query.set('page_size', String(params.page_size ?? 10))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.mode && params.mode !== 'all') query.set('mode', params.mode)
  if (params.mine) query.set('mine', '1')
  return query.toString()
}

/** Lists the agents of the connected Dify workspace; Dify is the only source. */
export async function listDifyAgents(
  params: DifyAgentListParams
): Promise<DifyAgentsPage> {
  const res = await api.get(
    `${BASE_URL}/dify/agents?${buildQuery(params)}`
  )
  return requireServerSuccess<ApiResponse<DifyAgentsPage>>(res.data).data
}

/** Explains how one agent can be reached (web app, service API, console). */
export async function getDifyAgentAccess(
  id: string
): Promise<DifyAgentAccess> {
  const res = await api.get(`${BASE_URL}/dify/agents/${id}/access`)
  return requireServerSuccess<ApiResponse<DifyAgentAccess>>(res.data).data
}


/**
 * True when the console has no usable Dify connection configured. The list
 * request then fails with a stable error code, which lets the page explain the
 * state instead of showing a generic failure.
 */
export function isDifyNotConfigured(error: unknown): boolean {
  const cause = (error as { cause?: unknown } | null | undefined)?.cause
  if (!cause || typeof cause !== 'object') return false
  return (cause as { code?: unknown }).code === 'dify_not_configured'
}
