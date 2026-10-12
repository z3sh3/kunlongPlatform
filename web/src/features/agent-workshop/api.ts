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
  Agent,
  AgentFormData,
  ApiResponse,
  ListParams,
  PagedData,
} from './types'

const BASE_URL = '/api/agent-workshop'

function buildQuery(params: ListParams): string {
  const query = new URLSearchParams()
  query.set('p', String(params.p ?? 1))
  query.set('page_size', String(params.page_size ?? 10))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.status !== undefined) {
    query.set('status', String(params.status))
  }
  if (params.mine) query.set('mine', '1')
  return query.toString()
}

export async function listAgents(
  params: ListParams
): Promise<PagedData<Agent>> {
  const res = await api.get(`${BASE_URL}/agents?${buildQuery(params)}`)
  return requireServerSuccess<ApiResponse<PagedData<Agent>>>(res.data).data
}

export async function getAgent(id: number): Promise<Agent> {
  const res = await api.get(`${BASE_URL}/agents/${id}`)
  return requireServerSuccess<ApiResponse<Agent>>(res.data).data
}

export async function createAgent(data: AgentFormData): Promise<Agent> {
  const res = await api.post(`${BASE_URL}/agents`, data)
  return requireServerSuccess<ApiResponse<Agent>>(res.data).data
}

export async function updateAgent(
  id: number,
  data: AgentFormData
): Promise<void> {
  const res = await api.put(`${BASE_URL}/agents/${id}`, data)
  requireServerSuccess(res.data)
}

export async function deleteAgent(id: number): Promise<void> {
  const res = await api.delete(`${BASE_URL}/agents/${id}`)
  requireServerSuccess(res.data)
}

export async function submitAgent(id: number): Promise<void> {
  const res = await api.post(`${BASE_URL}/agents/${id}/submit`)
  requireServerSuccess(res.data)
}

export async function adminListAgents(
  params: ListParams
): Promise<PagedData<Agent>> {
  const res = await api.get(`${BASE_URL}/admin/agents?${buildQuery(params)}`)
  return requireServerSuccess<ApiResponse<PagedData<Agent>>>(res.data).data
}

export async function adminReviewAgent(
  id: number,
  approve: boolean,
  comment = ''
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/agents/${id}/review`, {
    approve,
    comment,
  })
  requireServerSuccess(res.data)
}

export async function adminSetAgentOffline(
  id: number,
  offline: boolean
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/agents/${id}/offline`, {
    offline,
  })
  requireServerSuccess(res.data)
}
