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
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { DifyAgentsList } from '../components/dify-agents-list'
import type { DifyAgent } from '../types'

const CHAT_AGENT: DifyAgent = {
  id: 'app-chat',
  name: 'Support bot',
  description: 'Answers customer questions',
  mode: 'chat',
  icon_type: 'emoji',
  icon: '🤖',
  icon_background: '#FFEAD5',
  created_at: '2024-03-01T12:00:00Z',
  updated_at: '2024-03-02T12:00:00Z',
  console_path: '/app/app-chat/configuration',
}

const WORKFLOW_AGENT: DifyAgent = {
  ...CHAT_AGENT,
  id: 'app-flow',
  name: 'Intake flow',
  description: '',
  mode: 'workflow',
  console_path: '/app/app-flow/workflow',
}

function renderList(mode: 'browse' | 'mine') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <DifyAgentsList mode={mode} />
    </QueryClientProvider>
  )
}

function mockPage(items: DifyAgent[], total = items.length) {
  return vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      message: '',
      data: {
        page: 1,
        page_size: 10,
        total,
        items,
        console_base_url: 'http://console.example:8081',
      },
    },
  })
}

afterEach(() => {
  vi.restoreAllMocks()
})

test('renders Dify agents with their type and console deep link', async () => {
  mockPage([CHAT_AGENT, WORKFLOW_AGENT])

  renderList('browse')

  expect(await screen.findByText('Support bot')).toBeInTheDocument()
  expect(screen.getByText('Answers customer questions')).toBeInTheDocument()
  expect(screen.getByText('Chatbot')).toBeInTheDocument()
  expect(screen.getByText('Workflow')).toBeInTheDocument()

  const links = screen.getAllByRole('link', { name: /Open/ })
  expect(links).toHaveLength(2)
  expect(links[0]).toHaveAttribute(
    'href',
    'http://console.example:8081/app/app-chat/configuration'
  )
  expect(links[0]).toHaveAttribute('target', '_blank')
  expect(links[1]).toHaveAttribute(
    'href',
    'http://console.example:8081/app/app-flow/workflow'
  )
})

test('mine mode asks Dify for the apps created by the integration account', async () => {
  const get = mockPage([CHAT_AGENT])

  renderList('mine')

  expect(await screen.findByText('Support bot')).toBeInTheDocument()
  expect(get).toHaveBeenCalledWith(
    expect.stringContaining('/api/agent-workshop/dify/agents?')
  )
  expect(get.mock.calls[0][0]).toContain('mine=1')
})

test('explains that Dify is not connected when the console has no settings', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: false,
      code: 'dify_not_configured',
      message: 'Dify agent service is not configured',
    },
  })

  renderList('browse')

  expect(
    await screen.findByText('Dify is not connected')
  ).toBeInTheDocument()
  expect(
    screen.getByText('Contact an administrator to connect Dify')
  ).toBeInTheDocument()
})

test('shows an empty state when the Dify workspace has no apps', async () => {
  mockPage([], 0)

  renderList('browse')

  expect(
    await screen.findByText('No agents found in Dify')
  ).toBeInTheDocument()
})

test('explains how to use an agent with its web app and API endpoints', async () => {
  vi.spyOn(api, 'get').mockImplementation((url: unknown) => {
    if (String(url).includes('/access')) {
      return Promise.resolve({
        data: {
          success: true,
          data: {
            name: 'Support bot',
            mode: 'chat',
            webapp_url: 'http://console.example:8081/chat/tok123',
            api_base_url: 'http://console.example:8081/v1',
            console_path: '/app/app-chat/configuration',
          },
        },
      })
    }
    return Promise.resolve({
      data: {
        success: true,
        data: {
          page: 1,
          page_size: 10,
          total: 1,
          items: [CHAT_AGENT],
          console_base_url: 'http://console.example:8081',
        },
      },
    })
  })

  renderList('browse')

  expect(await screen.findByText('Support bot')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'How to use' }))

  expect(await screen.findByText('How to use this agent')).toBeInTheDocument()
  // The access details arrive asynchronously after the dialog opens.
  expect(await screen.findByText('Use in the web app')).toBeInTheDocument()
  expect(
    screen.getByText('http://console.example:8081/chat/tok123')
  ).toBeInTheDocument()
  expect(screen.getByText('Call through the API')).toBeInTheDocument()
  expect(
    screen.getByText('http://console.example:8081/v1')
  ).toBeInTheDocument()
  expect(
    screen.getByText('Orchestrate in the Dify console')
  ).toBeInTheDocument()
})
