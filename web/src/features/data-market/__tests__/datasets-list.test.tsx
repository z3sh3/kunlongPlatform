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

import { PUBLISH_STATUS } from '../constants'
import { DatasetsList } from '../components/datasets-list'
import type { Dataset } from '../types'

const datasets: Dataset[] = [
  {
    id: 1,
    user_id: 9,
    name: 'open-images',
    description: 'A public CV dataset',
    category: 'cv',
    format: 'jsonl',
    tags: '',
    file_url: '',
    cover_url: '',
    size_bytes: 1024,
    price_mode: 0,
    unit_price: 0,
    status: PUBLISH_STATUS.PUBLISHED,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
]

function renderList(mode: 'browse' | 'mine' | 'admin') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <DatasetsList mode={mode} />
    </QueryClientProvider>
  )
}

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

test('browse mode renders published datasets with an apply action', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'alice', role: 1 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 1, items: datasets },
    },
  })

  renderList('browse')

  expect(await screen.findByText('open-images')).toBeInTheDocument()
  expect(screen.getByText('A public CV dataset')).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Apply for access' })
  ).toBeInTheDocument()
  expect(screen.queryByText('Create dataset')).not.toBeInTheDocument()
})

test('admin mode renders draft status and publish action for administrators', async () => {
  useAuthStore.getState().auth.setUser({ id: 9, username: 'root', role: 100 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        page: 1,
        page_size: 10,
        total: 1,
        items: [{ ...datasets[0], status: PUBLISH_STATUS.DRAFT }],
      },
    },
  })

  renderList('admin')

  expect(await screen.findByText('open-images')).toBeInTheDocument()
  expect(screen.getByText('Draft')).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Publish' })
  ).toBeInTheDocument()
})

test('mine mode renders the create entry for the owner', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'alice', role: 1 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 0, items: [] },
    },
  })

  renderList('mine')

  expect(await screen.findByText('No datasets yet')).toBeInTheDocument()
  expect(screen.getByText('Create dataset')).toBeInTheDocument()
})
