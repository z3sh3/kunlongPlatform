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

import { JOBS } from '../fixtures/jobs'
import { JobsList } from '../components/jobs-list'

function renderList(mode: 'mine' | 'admin') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <JobsList mode={mode} />
    </QueryClientProvider>
  )
}

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

test('mine mode renders the own draft job with edit and submit actions', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'alice', role: 1 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 1, items: [JOBS.draft] },
    },
  })

  renderList('mine')

  expect(await screen.findByText('my-lora-job')).toBeInTheDocument()
  expect(screen.getByText('gpt-4o-mini · lora')).toBeInTheDocument()
  expect(screen.getByText('Draft')).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Edit' })
  ).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Submit' })
  ).toBeInTheDocument()
})

test('mine mode offers cancel for a submitted job', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'alice', role: 1 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 1, items: [JOBS.submitted] },
    },
  })

  renderList('mine')

  expect(await screen.findByText('queued-sft-job')).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Cancel' })
  ).toBeInTheDocument()
  expect(screen.queryByText('Edit')).not.toBeInTheDocument()
})

test('admin mode shows the status update control', async () => {
  useAuthStore.getState().auth.setUser({ id: 9, username: 'root', role: 100 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 2, items: JOBS.all },
    },
  })

  renderList('admin')

  expect(await screen.findByText('my-lora-job')).toBeInTheDocument()
  expect(
    screen.getAllByRole('button', { name: 'Save' }).length
  ).toBeGreaterThan(0)
  expect(screen.queryByText('Create fine-tuning job')).not.toBeInTheDocument()
})
