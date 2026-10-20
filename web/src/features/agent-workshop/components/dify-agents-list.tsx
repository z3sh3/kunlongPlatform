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
import { useQuery } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { ExternalLink, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ServerPagination, StaticDataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useDebounce } from '@/hooks'
import { getAgentDevUrl } from '@/lib/constants'

import { isDifyNotConfigured, listDifyAgents } from '../api'
import type { DifyAgent } from '../types'
import { AgentAccessDialog } from './agent-access-dialog'

type DifyAgentsListProps = {
  mode: 'browse' | 'mine'
}

/**
 * Type filter options. Dify's app list does not accept rag-pipeline as a
 * filter value, so it is absent here; rag-pipeline apps still show up in the
 * unfiltered listing with their own badge label below.
 */
const AGENT_TYPE_OPTIONS = [
  { value: 'all', labelKey: 'All types' },
  { value: 'chat', labelKey: 'Chatbot' },
  { value: 'advanced-chat', labelKey: 'Chatflow' },
  { value: 'agent-chat', labelKey: 'Agent Chat' },
  { value: 'agent', labelKey: 'Agent' },
  { value: 'workflow', labelKey: 'Workflow' },
  { value: 'completion', labelKey: 'Text Completion' },
  { value: 'channel', labelKey: 'Channel' },
]

const AGENT_MODE_LABELS: Record<string, string> = {
  chat: 'Chatbot',
  'advanced-chat': 'Chatflow',
  'agent-chat': 'Agent Chat',
  agent: 'Agent',
  workflow: 'Workflow',
  completion: 'Text Completion',
  channel: 'Channel',
  'rag-pipeline': 'RAG Pipeline',
}

export function DifyAgentsList(props: DifyAgentsListProps) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [keyword, setKeyword] = useState('')
  const [agentType, setAgentType] = useState('all')
  const [accessAgent, setAccessAgent] = useState<DifyAgent | null>(null)
  const debouncedKeyword = useDebounce(keyword.trim(), 400)

  const agentsQuery = useQuery({
    queryKey: [
      'agent-workshop',
      'dify-agents',
      props.mode,
      page,
      pageSize,
      agentType,
      debouncedKeyword,
    ],
    queryFn: () =>
      listDifyAgents({
        p: page,
        page_size: pageSize,
        keyword: debouncedKeyword,
        mode: agentType,
        mine: props.mode === 'mine',
      }),
    retry: false,
  })

  const items = agentsQuery.data?.items ?? []
  const consoleBaseURL =
    agentsQuery.data?.console_base_url || getAgentDevUrl()

  const columns = [
    {
      id: 'name',
      header: t('Name'),
      cell: (agent: DifyAgent) => (
        <div className='flex min-w-0 items-center gap-3'>
          <DifyAgentIcon agent={agent} />
          <div className='min-w-0'>
            <p className='truncate font-medium'>{agent.name}</p>
            {agent.description ? (
              <p className='text-muted-foreground truncate text-xs'>
                {agent.description}
              </p>
            ) : null}
          </div>
        </div>
      ),
    },
    {
      id: 'mode',
      header: t('Type'),
      cell: (agent: DifyAgent) => (
        <Badge variant='outline'>
          {t(AGENT_MODE_LABELS[agent.mode] ?? 'Agent')}
        </Badge>
      ),
    },
    {
      id: 'updated_at',
      header: t('Updated'),
      cell: (agent: DifyAgent) =>
        agent.updated_at
          ? dayjs(agent.updated_at).format('YYYY-MM-DD')
          : '-',
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cell: (agent: DifyAgent) => (
        <div className='flex justify-end'>
          <Button
            variant='ghost'
            size='sm'
            onClick={() => setAccessAgent(agent)}
          >
            {t('How to use')}
          </Button>
          <a
            href={`${consoleBaseURL}${agent.console_path}`}
            target='_blank'
            rel='noopener noreferrer'
            className={buttonVariants({ variant: 'ghost', size: 'sm' })}
          >
            {t('Open')}
            <ExternalLink />
          </a>
        </div>
      ),
    },
  ]

  if (agentsQuery.isError) {
    if (isDifyNotConfigured(agentsQuery.error)) {
      return (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>{t('Dify is not connected')}</EmptyTitle>
            <EmptyDescription>
              {t('Contact an administrator to connect Dify')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      )
    }
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('Failed to load agents from Dify')}</EmptyTitle>
          <EmptyDescription>
            {t('Check the Dify service and the connection settings')}
          </EmptyDescription>
        </EmptyHeader>
        <Button variant='outline' size='sm' onClick={() => agentsQuery.refetch()}>
          <RefreshCw />
          {t('Retry')}
        </Button>
      </Empty>
    )
  }

  return (
    <div className='space-y-4'>
      <div className='flex flex-wrap items-center gap-2'>
        <Input
          value={keyword}
          placeholder={t('Search agents...')}
          aria-label={t('Search agents...')}
          className='w-full sm:max-w-xs'
          onChange={(event) => {
            setKeyword(event.target.value)
            setPage(1)
          }}
        />
        <Select
          value={agentType}
          onValueChange={(value) => {
            if (value === null) return
            setAgentType(value)
            setPage(1)
          }}
        >
          <SelectTrigger className='w-full sm:w-48'>
            <SelectValue placeholder={t('All types')} />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            {AGENT_TYPE_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {t(option.labelKey)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <StaticDataTable
        columns={columns}
        data={items}
        getRowKey={(agent) => agent.id}
        empty={agentsQuery.isSuccess && items.length === 0}
        emptyContent={
          agentsQuery.isSuccess
            ? t('No agents found in Dify')
            : t('Loading...')
        }
      />
      {agentsQuery.data ? (
        <ServerPagination
          page={page}
          pageSize={pageSize}
          total={agentsQuery.data.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          disabled={agentsQuery.isFetching}
        />
      ) : null}
      <AgentAccessDialog
        open={accessAgent !== null}
        onOpenChange={(open) => {
          if (!open) setAccessAgent(null)
        }}
        agent={accessAgent}
        consoleBaseURL={consoleBaseURL}
      />
    </div>
  )
}

function DifyAgentIcon({ agent }: { agent: DifyAgent }) {
  if (agent.icon_type === 'image' && agent.icon) {
    return (
      <img
        src={agent.icon}
        alt=''
        className='size-8 shrink-0 rounded-full object-cover'
      />
    )
  }
  return (
    <span
      aria-hidden='true'
      className='flex size-8 shrink-0 items-center justify-center rounded-full text-base'
      style={{ backgroundColor: agent.icon_background || undefined }}
    >
      {agent.icon || '🤖'}
    </span>
  )
}
