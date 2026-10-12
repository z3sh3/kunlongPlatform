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
  AdminJobStatusPayload,
  ApiResponse,
  FinetuneJob,
  FinetuneJobFormData,
  ListParams,
  PagedData,
} from './types'

const BASE_URL = '/api/model-training'

function buildQuery(params: ListParams, withUserId = false): string {
  const query = new URLSearchParams()
  query.set('p', String(params.p ?? 1))
  query.set('page_size', String(params.page_size ?? 10))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.status !== undefined) {
    query.set('status', String(params.status))
  }
  if (withUserId && params.userId !== undefined) {
    query.set('user_id', String(params.userId))
  }
  return query.toString()
}

export async function listJobs(
  params: ListParams
): Promise<PagedData<FinetuneJob>> {
  const res = await api.get(`${BASE_URL}/jobs?${buildQuery(params)}`)
  return requireServerSuccess<ApiResponse<PagedData<FinetuneJob>>>(res.data).data
}

export async function getJob(id: number): Promise<FinetuneJob> {
  const res = await api.get(`${BASE_URL}/jobs/${id}`)
  return requireServerSuccess<ApiResponse<FinetuneJob>>(res.data).data
}

export async function createJob(
  data: FinetuneJobFormData
): Promise<FinetuneJob> {
  const res = await api.post(`${BASE_URL}/jobs`, data)
  return requireServerSuccess<ApiResponse<FinetuneJob>>(res.data).data
}

export async function updateJob(
  id: number,
  data: FinetuneJobFormData
): Promise<void> {
  const res = await api.put(`${BASE_URL}/jobs/${id}`, data)
  requireServerSuccess(res.data)
}

export async function deleteJob(id: number): Promise<void> {
  const res = await api.delete(`${BASE_URL}/jobs/${id}`)
  requireServerSuccess(res.data)
}

export async function submitJob(id: number): Promise<void> {
  const res = await api.post(`${BASE_URL}/jobs/${id}/submit`)
  requireServerSuccess(res.data)
}

export async function cancelJob(id: number): Promise<void> {
  const res = await api.post(`${BASE_URL}/jobs/${id}/cancel`)
  requireServerSuccess(res.data)
}

export async function adminListJobs(
  params: ListParams
): Promise<PagedData<FinetuneJob>> {
  const res = await api.get(
    `${BASE_URL}/admin/jobs?${buildQuery(params, true)}`
  )
  return requireServerSuccess<ApiResponse<PagedData<FinetuneJob>>>(res.data).data
}

export async function adminUpdateJobStatus(
  id: number,
  payload: AdminJobStatusPayload
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/jobs/${id}/status`, payload)
  requireServerSuccess(res.data)
}
