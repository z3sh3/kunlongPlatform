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
  Dataset,
  DatasetFormData,
  DatasetGrant,
  ListParams,
  PagedData,
} from './types'

const BASE_URL = '/api/data-market'

function buildQuery(params: ListParams): string {
  const query = new URLSearchParams()
  query.set('p', String(params.p ?? 1))
  query.set('page_size', String(params.page_size ?? 10))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.status !== undefined) query.set('status', String(params.status))
  if (params.mine) query.set('mine', '1')
  return query.toString()
}

export async function listDatasets(params: ListParams): Promise<PagedData<Dataset>> {
  const res = await api.get(`${BASE_URL}/datasets?${buildQuery(params)}`)
  return requireServerSuccess<ApiResponse<PagedData<Dataset>>>(res.data).data
}

export async function getDataset(id: number): Promise<Dataset> {
  const res = await api.get(`${BASE_URL}/datasets/${id}`)
  return requireServerSuccess<ApiResponse<Dataset>>(res.data).data
}

export async function createDataset(
  data: DatasetFormData
): Promise<Dataset> {
  const res = await api.post(`${BASE_URL}/datasets`, data)
  return requireServerSuccess<ApiResponse<Dataset>>(res.data).data
}

export async function updateDataset(
  id: number,
  data: DatasetFormData
): Promise<void> {
  const res = await api.put(`${BASE_URL}/datasets/${id}`, data)
  requireServerSuccess(res.data)
}

export async function deleteDataset(id: number): Promise<void> {
  const res = await api.delete(`${BASE_URL}/datasets/${id}`)
  requireServerSuccess(res.data)
}

export async function applyDataset(id: number): Promise<DatasetGrant> {
  const res = await api.post(`${BASE_URL}/datasets/${id}/apply`)
  return requireServerSuccess<ApiResponse<DatasetGrant>>(res.data).data
}

export async function listMyGrants(
  params: ListParams
): Promise<PagedData<DatasetGrant>> {
  const query = new URLSearchParams()
  query.set('p', String(params.p ?? 1))
  query.set('page_size', String(params.page_size ?? 10))
  const res = await api.get(`${BASE_URL}/grants/my?${query.toString()}`)
  return requireServerSuccess<ApiResponse<PagedData<DatasetGrant>>>(res.data).data
}

export async function adminListDatasets(
  params: ListParams
): Promise<PagedData<Dataset>> {
  const query = new URLSearchParams()
  query.set('p', String(params.p ?? 1))
  query.set('page_size', String(params.page_size ?? 10))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.status !== undefined) {
    query.set('status', String(params.status))
  }
  const res = await api.get(`${BASE_URL}/admin/datasets?${query.toString()}`)
  return requireServerSuccess<ApiResponse<PagedData<Dataset>>>(res.data).data
}

export async function adminUpdateDatasetStatus(
  id: number,
  status: number
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/datasets/${id}/status`, {
    status,
  })
  requireServerSuccess(res.data)
}

export async function adminListGrants(
  params: ListParams
): Promise<PagedData<DatasetGrant>> {
  const query = new URLSearchParams()
  query.set('p', String(params.p ?? 1))
  query.set('page_size', String(params.page_size ?? 10))
  if (params.status !== undefined) {
    query.set('status', String(params.status))
  }
  const res = await api.get(`${BASE_URL}/admin/grants?${query.toString()}`)
  return requireServerSuccess<ApiResponse<PagedData<DatasetGrant>>>(res.data).data
}

export async function adminReviewGrant(
  id: number,
  status: number,
  expiresAt = 0,
  remark = ''
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/grants/${id}/review`, {
    status,
    expires_at: expiresAt,
    remark,
  })
  requireServerSuccess(res.data)
}
