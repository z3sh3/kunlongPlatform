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
import { render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { CoursesList } from '../components/courses-list'
import { EnrollmentsList } from '../components/enrollments-list'
import {
  COURSE_CATEGORY,
  PUBLISH_STATUS,
  REVIEW_STATUS,
} from '../constants'
import type { TrainingCourse, TrainingEnrollment } from '../types'

const course: TrainingCourse = {
  id: 1,
  title: 'LLM engineer certification',
  category: COURSE_CATEGORY.CERTIFICATION,
  description: 'A certification course',
  content: '',
  schedule: '',
  capacity: 30,
  price_mode: 0,
  price: 0,
  status: PUBLISH_STATUS.PUBLISHED,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const enrollment: TrainingEnrollment = {
  id: 1,
  course_id: 1,
  user_id: 2,
  status: REVIEW_STATUS.PENDING,
  remark: '',
  created_at: '2026-01-02T00:00:00Z',
  updated_at: '2026-01-02T00:00:00Z',
  course,
}

function renderCourses(mode: 'browse' | 'admin') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <CoursesList mode={mode} />
    </QueryClientProvider>
  )
}

function renderEnrollments(mode: 'mine' | 'admin') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <EnrollmentsList mode={mode} />
    </QueryClientProvider>
  )
}

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

test('browse mode renders a published certification course with enroll action', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'alice', role: 1 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 1, items: [course] },
    },
  })

  renderCourses('browse')

  expect(
    await screen.findByText('LLM engineer certification')
  ).toBeInTheDocument()
  expect(screen.getByText('Certification training')).toBeInTheDocument()
  expect(screen.getByText('30')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Enroll' })).toBeInTheDocument()
})

test('admin mode shows publish action for a draft course', async () => {
  useAuthStore.getState().auth.setUser({ id: 9, username: 'root', role: 100 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        page: 1,
        page_size: 10,
        total: 1,
        items: [{ ...course, status: PUBLISH_STATUS.DRAFT }],
      },
    },
  })

  renderCourses('admin')

  expect(
    await screen.findByText('LLM engineer certification')
  ).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Publish' })
  ).toBeInTheDocument()
})

test('admin enrollments list offers approve and reject for pending records', async () => {
  useAuthStore.getState().auth.setUser({ id: 9, username: 'root', role: 100 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 1, items: [enrollment] },
    },
  })

  renderEnrollments('admin')

  expect(
    await screen.findByText('LLM engineer certification')
  ).toBeInTheDocument()
  expect(screen.getByText('Pending')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Approve' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Reject' })).toBeInTheDocument()
})
