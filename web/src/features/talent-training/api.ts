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
  CourseFormData,
  DemoSeedResult,
  ListParams,
  PagedData,
  TrainingCourse,
  TrainingEnrollment,
} from './types'

const BASE_URL = '/api/talent-training'

function buildQuery(params: ListParams): string {
  const query = new URLSearchParams()
  query.set('p', String(params.p ?? 1))
  query.set('page_size', String(params.page_size ?? 10))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.status !== undefined) {
    query.set('status', String(params.status))
  }
  if (params.courseId !== undefined) {
    query.set('course_id', String(params.courseId))
  }
  return query.toString()
}

export async function listCourses(
  params: ListParams
): Promise<PagedData<TrainingCourse>> {
  const res = await api.get(`${BASE_URL}/courses?${buildQuery(params)}`)
  return requireServerSuccess<ApiResponse<PagedData<TrainingCourse>>>(res.data).data
}

export async function getCourse(id: number): Promise<TrainingCourse> {
  const res = await api.get(`${BASE_URL}/courses/${id}`)
  return requireServerSuccess<ApiResponse<TrainingCourse>>(res.data).data
}

export async function listMyEnrollments(
  params: ListParams
): Promise<PagedData<TrainingEnrollment>> {
  const res = await api.get(
    `${BASE_URL}/enrollments/my?${buildQuery(params)}`
  )
  return requireServerSuccess<ApiResponse<PagedData<TrainingEnrollment>>>(res.data).data
}

export async function enroll(id: number): Promise<TrainingEnrollment> {
  const res = await api.post(`${BASE_URL}/courses/${id}/enroll`)
  return requireServerSuccess<ApiResponse<TrainingEnrollment>>(res.data).data
}

export async function adminListCourses(
  params: ListParams
): Promise<PagedData<TrainingCourse>> {
  const res = await api.get(`${BASE_URL}/admin/courses?${buildQuery(params)}`)
  return requireServerSuccess<ApiResponse<PagedData<TrainingCourse>>>(res.data).data
}

export async function adminCreateCourse(
  data: CourseFormData
): Promise<TrainingCourse> {
  const res = await api.post(`${BASE_URL}/admin/courses`, data)
  return requireServerSuccess<ApiResponse<TrainingCourse>>(res.data).data
}

export async function adminUpdateCourse(
  id: number,
  data: CourseFormData
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/courses/${id}`, data)
  requireServerSuccess(res.data)
}

export async function adminDeleteCourse(id: number): Promise<void> {
  const res = await api.delete(`${BASE_URL}/admin/courses/${id}`)
  requireServerSuccess(res.data)
}

export async function adminUpdateCourseStatus(
  id: number,
  status: number
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/courses/${id}/status`, {
    status,
  })
  requireServerSuccess(res.data)
}

/** Publishes the bundled demo courses and one enrollment per course. */
export async function adminSeedDemoCourses(): Promise<DemoSeedResult> {
  const res = await api.post(`${BASE_URL}/admin/demo-courses`)
  return requireServerSuccess<ApiResponse<DemoSeedResult>>(res.data).data
}

export async function adminListEnrollments(
  params: ListParams
): Promise<PagedData<TrainingEnrollment>> {
  const res = await api.get(
    `${BASE_URL}/admin/enrollments?${buildQuery(params)}`
  )
  return requireServerSuccess<ApiResponse<PagedData<TrainingEnrollment>>>(res.data).data
}

export async function adminReviewEnrollment(
  id: number,
  status: number,
  remark = ''
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/enrollments/${id}/review`, {
    status,
    remark,
  })
  requireServerSuccess(res.data)
}
