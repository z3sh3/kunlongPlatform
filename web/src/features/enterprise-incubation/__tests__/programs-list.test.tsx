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
import userEvent from '@testing-library/user-event'
import { Toaster, toast } from 'sonner'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { ApplicationsList } from '../components/applications-list'
import { ProgramsList } from '../components/programs-list'
import {
  PROGRAM_TYPE,
  PUBLISH_STATUS,
  REVIEW_STATUS,
} from '../constants'
import type { IncubationApplication, IncubationProgram } from '../types'

const program: IncubationProgram = {
  id: 1,
  title: 'AI startup camp',
  type: PROGRAM_TYPE.INCUBATION,
  summary: 'Incubation track intro',
  cover_url: '',
  description: 'Incubation for AI startups',
  requirements: '',
  status: PUBLISH_STATUS.PUBLISHED,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const application: IncubationApplication = {
  id: 1,
  program_id: 1,
  user_id: 2,
  company_name: 'ACME',
  contact_name: 'Wang',
  contact_phone: '13800000000',
  description: '',
  status: REVIEW_STATUS.PENDING,
  review_comment: '',
  created_at: '2026-01-02T00:00:00Z',
  updated_at: '2026-01-02T00:00:00Z',
  program,
}

function renderPrograms(mode: 'browse' | 'admin') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <ProgramsList mode={mode} />
      <Toaster />
    </QueryClientProvider>
  )
}

function renderApplications(mode: 'mine' | 'admin') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <ApplicationsList mode={mode} />
    </QueryClientProvider>
  )
}

afterEach(() => {
  toast.dismiss()
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

test('browse mode renders a published program with an apply action', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'alice', role: 1 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 1, items: [program] },
    },
  })

  renderPrograms('browse')

  expect(await screen.findByText('AI startup camp')).toBeInTheDocument()
  expect(screen.getByText('Enterprise incubation')).toBeInTheDocument()
  // Card layout contract: the short introduction and the placeholder cover are
  // rendered instead of a table row.
  expect(screen.getByText('Incubation track intro')).toBeInTheDocument()
  expect(screen.getByText('AI')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Apply' })).toBeInTheDocument()
})

test('admin mode shows publish action for a draft program', async () => {
  useAuthStore.getState().auth.setUser({ id: 9, username: 'root', role: 100 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        page: 1,
        page_size: 10,
        total: 1,
        items: [{ ...program, status: PUBLISH_STATUS.DRAFT }],
      },
    },
  })

  renderPrograms('admin')

  expect(await screen.findByText('AI startup camp')).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Publish' })
  ).toBeInTheDocument()
})

test('admin applications list renders company info with review actions', async () => {
  useAuthStore.getState().auth.setUser({ id: 9, username: 'root', role: 100 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 1, items: [application] },
    },
  })

  renderApplications('admin')

  expect(await screen.findByText('ACME')).toBeInTheDocument()
  expect(screen.getByText('AI startup camp')).toBeInTheDocument()
  expect(screen.getByText('Pending')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Start review' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Approve' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Reject' })).toBeInTheDocument()
})

test('admin can generate the demo cases and sees the created count', async () => {
  useAuthStore.getState().auth.setUser({ id: 9, username: 'root', role: 100 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 0, items: [] },
    },
  })
  const post = vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      success: true,
      data: { created: [program], existing: [], applications: 1 },
    },
  })

  renderPrograms('admin')

  const user = userEvent.setup()
  await user.click(
    await screen.findByRole('button', { name: 'Generate demo cases' })
  )

  await waitFor(() =>
    expect(post).toHaveBeenCalledWith(
      '/api/enterprise-incubation/admin/demo-programs'
    )
  )
  expect(
    await screen.findByText('Created 1 demo case(s)')
  ).toBeInTheDocument()
})
