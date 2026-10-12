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

import { AgentsList } from '../components/agents-list'
import { AGENT_STATUS } from '../constants'
import type { Agent } from '../types'

const published: Agent = {
  id: 1,
  user_id: 9,
  name: 'coding-helper',
  description: 'A coding assistant agent',
  icon: '',
  category: 'coding',
  config: '',
  price_mode: 0,
  unit_price: 0,
  status: AGENT_STATUS.PUBLISHED,
  review_comment: '',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function renderList(mode: 'browse' | 'mine' | 'admin') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <AgentsList mode={mode} />
    </QueryClientProvider>
  )
}

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

test('browse mode renders published agents without management actions', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'alice', role: 1 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: 1, items: [published] },
    },
  })

  renderList('browse')

  expect(await screen.findByText('coding-helper')).toBeInTheDocument()
  expect(
    screen.getByText('A coding assistant agent')
  ).toBeInTheDocument()
  expect(screen.queryByText('Create agent')).not.toBeInTheDocument()
  expect(screen.queryByText('Edit')).not.toBeInTheDocument()
})

test('mine mode shows submit for review on a draft agent', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'alice', role: 1 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        page: 1,
        page_size: 10,
        total: 1,
        items: [{ ...published, status: AGENT_STATUS.DRAFT }],
      },
    },
  })

  renderList('mine')

  expect(await screen.findByText('coding-helper')).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Submit for review' })
  ).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
})

test('admin mode offers approve and reject for a reviewing agent', async () => {
  useAuthStore.getState().auth.setUser({ id: 9, username: 'root', role: 100 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        page: 1,
        page_size: 10,
        total: 1,
        items: [{ ...published, status: AGENT_STATUS.REVIEWING }],
      },
    },
  })

  renderList('admin')

  expect(await screen.findByText('coding-helper')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Approve' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Reject' })).toBeInTheDocument()
})
