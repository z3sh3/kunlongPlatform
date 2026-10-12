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
  ApplicationFormData,
  IncubationApplication,
  IncubationProgram,
  ListParams,
  PagedData,
  ProgramFormData,
} from './types'

const BASE_URL = '/api/enterprise-incubation'

function buildQuery(params: ListParams): string {
  const query = new URLSearchParams()
  query.set('p', String(params.p ?? 1))
  query.set('page_size', String(params.page_size ?? 10))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.status !== undefined) {
    query.set('status', String(params.status))
  }
  if (params.programId !== undefined) {
    query.set('program_id', String(params.programId))
  }
  return query.toString()
}

export async function listPrograms(
  params: ListParams
): Promise<PagedData<IncubationProgram>> {
  const res = await api.get(`${BASE_URL}/programs?${buildQuery(params)}`)
  return requireServerSuccess<ApiResponse<PagedData<IncubationProgram>>>(res.data).data
}

export async function getProgram(id: number): Promise<IncubationProgram> {
  const res = await api.get(`${BASE_URL}/programs/${id}`)
  return requireServerSuccess<ApiResponse<IncubationProgram>>(res.data).data
}

export async function listMyApplications(
  params: ListParams
): Promise<PagedData<IncubationApplication>> {
  const res = await api.get(
    `${BASE_URL}/applications/my?${buildQuery(params)}`
  )
  return requireServerSuccess<ApiResponse<PagedData<IncubationApplication>>>(res.data).data
}

export async function createApplication(
  data: ApplicationFormData
): Promise<IncubationApplication> {
  const res = await api.post(`${BASE_URL}/applications`, data)
  return requireServerSuccess<ApiResponse<IncubationApplication>>(res.data).data
}

export async function adminListPrograms(
  params: ListParams
): Promise<PagedData<IncubationProgram>> {
  const res = await api.get(
    `${BASE_URL}/admin/programs?${buildQuery(params)}`
  )
  return requireServerSuccess<ApiResponse<PagedData<IncubationProgram>>>(res.data).data
}

export async function adminCreateProgram(
  data: ProgramFormData
): Promise<IncubationProgram> {
  const res = await api.post(`${BASE_URL}/admin/programs`, data)
  return requireServerSuccess<ApiResponse<IncubationProgram>>(res.data).data
}

export async function adminUpdateProgram(
  id: number,
  data: ProgramFormData
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/programs/${id}`, data)
  requireServerSuccess(res.data)
}

export async function adminDeleteProgram(id: number): Promise<void> {
  const res = await api.delete(`${BASE_URL}/admin/programs/${id}`)
  requireServerSuccess(res.data)
}

export async function adminUpdateProgramStatus(
  id: number,
  status: number
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/programs/${id}/status`, {
    status,
  })
  requireServerSuccess(res.data)
}

export async function adminListApplications(
  params: ListParams
): Promise<PagedData<IncubationApplication>> {
  const res = await api.get(
    `${BASE_URL}/admin/applications?${buildQuery(params)}`
  )
  return requireServerSuccess<ApiResponse<PagedData<IncubationApplication>>>(res.data).data
}

export async function adminReviewApplication(
  id: number,
  status: number,
  comment = ''
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/applications/${id}/review`, {
    status,
    comment,
  })
  requireServerSuccess(res.data)
}
