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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ServerPagination, StaticDataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { handleServerError } from '@/lib/handle-server-error'

import {
  adminListAgents,
  adminReviewAgent,
  adminSetAgentOffline,
  deleteAgent,
  listAgents,
  submitAgent,
} from '../api'
import {
  AGENT_STATUS,
  AGENT_STATUS_OPTIONS,
  getOptionLabelKey,
} from '../constants'
import type { Agent } from '../types'
import { AgentFormDialog } from './agent-form-dialog'

type AgentsListMode = 'browse' | 'mine' | 'admin'

type AgentsListProps = {
  mode: AgentsListMode
}

export function AgentsList(props: AgentsListProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Agent | null>(null)

  const isMine = props.mode === 'mine'
  const isAdmin = props.mode === 'admin'

  const agentsQuery = useQuery({
    queryKey: ['agent-workshop', 'agents', props.mode, page, pageSize],
    queryFn: () =>
      isAdmin
        ? adminListAgents({ p: page, page_size: pageSize })
        : listAgents({ p: page, page_size: pageSize, mine: isMine }),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['agent-workshop', 'agents'] })
  }

  const submitMutation = useMutation({
    mutationFn: (id: number) => submitAgent(id),
    onSuccess: () => {
      toast.success(t('Agent submitted for review'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteAgent(id),
    onSuccess: () => {
      toast.success(t('Agent deleted'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const reviewMutation = useMutation({
    mutationFn: (values: { id: number; approve: boolean }) =>
      adminReviewAgent(values.id, values.approve),
    onSuccess: (_, values) => {
      toast.success(values.approve ? t('Agent approved') : t('Agent rejected'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const offlineMutation = useMutation({
    mutationFn: (values: { id: number; offline: boolean }) =>
      adminSetAgentOffline(values.id, values.offline),
    onSuccess: () => {
      toast.success(t('Agent availability updated'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const renderMineActions = (agent: Agent) => {
    const actions = []
    if (agent.status === AGENT_STATUS.DRAFT) {
      actions.push(
        <Button
          key='edit'
          variant='ghost'
          size='sm'
          onClick={() => {
            setEditing(agent)
            setFormOpen(true)
          }}
        >
          {t('Edit')}
        </Button>,
        <Button
          key='submit'
          variant='ghost'
          size='sm'
          onClick={() => submitMutation.mutate(agent.id)}
          disabled={submitMutation.isPending}
        >
          {t('Submit for review')}
        </Button>
      )
    }
    if (agent.status === AGENT_STATUS.DRAFT || agent.status === AGENT_STATUS.OFFLINE) {
      actions.push(
        <Button
          key='delete'
          variant='ghost'
          size='sm'
          className='text-destructive hover:text-destructive'
          onClick={() => deleteMutation.mutate(agent.id)}
          disabled={deleteMutation.isPending}
        >
          {t('Delete')}
        </Button>
      )
    }
    return actions
  }

  const renderAdminActions = (agent: Agent) => {
    const actions = []
    if (agent.status === AGENT_STATUS.REVIEWING) {
      actions.push(
        <Button
          key='approve'
          variant='ghost'
          size='sm'
          onClick={() => reviewMutation.mutate({ id: agent.id, approve: true })}
          disabled={reviewMutation.isPending}
        >
          {t('Approve')}
        </Button>,
        <Button
          key='reject'
          variant='ghost'
          size='sm'
          className='text-destructive hover:text-destructive'
          onClick={() => reviewMutation.mutate({ id: agent.id, approve: false })}
          disabled={reviewMutation.isPending}
        >
          {t('Reject')}
        </Button>
      )
    }
    if (agent.status === AGENT_STATUS.PUBLISHED) {
      actions.push(
        <Button
          key='offline'
          variant='ghost'
          size='sm'
          onClick={() => offlineMutation.mutate({ id: agent.id, offline: true })}
          disabled={offlineMutation.isPending}
        >
          {t('Take offline')}
        </Button>
      )
    }
    if (agent.status === AGENT_STATUS.OFFLINE) {
      actions.push(
        <Button
          key='restore'
          variant='ghost'
          size='sm'
          onClick={() =>
            offlineMutation.mutate({ id: agent.id, offline: false })
          }
          disabled={offlineMutation.isPending}
        >
          {t('Restore')}
        </Button>
      )
    }
    return actions
  }

  const columns = [
    {
      id: 'name',
      header: t('Name'),
      cell: (agent: Agent) => (
        <div className='min-w-0'>
          <p className='truncate font-medium'>{agent.name}</p>
          {agent.description ? (
            <p className='text-muted-foreground truncate text-xs'>
              {agent.description}
            </p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'category',
      header: t('Category'),
      cell: (agent: Agent) => agent.category || '-',
    },
    ...(isAdmin || isMine
      ? [
          {
            id: 'status',
            header: t('Status'),
            cell: (agent: Agent) => (
              <div className='flex flex-col gap-1'>
                <Badge variant='outline'>
                  {t(
                    getOptionLabelKey(AGENT_STATUS_OPTIONS, agent.status) ??
                      'Draft'
                  )}
                </Badge>
                {isMine && agent.review_comment ? (
                  <span className='text-muted-foreground max-w-[200px] truncate text-xs'>
                    {agent.review_comment}
                  </span>
                ) : null}
              </div>
            ),
          },
        ]
      : []),
    {
      id: 'created_at',
      header: t('Created'),
      cell: (agent: Agent) =>
        agent.created_at ? dayjs(agent.created_at).format('YYYY-MM-DD') : '-',
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cell: (agent: Agent) => (
        <div className='flex justify-end gap-1'>
          {isMine ? renderMineActions(agent) : null}
          {isAdmin ? renderAdminActions(agent) : null}
        </div>
      ),
    },
  ]

  const items = agentsQuery.data?.items ?? []

  return (
    <div className='space-y-4'>
      {isMine ? (
        <div className='flex justify-end'>
          <Button
            onClick={() => {
              setEditing(null)
              setFormOpen(true)
            }}
          >
            <Plus />
            {t('Create agent')}
          </Button>
        </div>
      ) : null}
      <StaticDataTable
        columns={columns}
        data={items}
        getRowKey={(agent) => agent.id}
        empty={agentsQuery.isSuccess && items.length === 0}
        emptyContent={t('No agents yet')}
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
      <AgentFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        agent={editing}
      />
    </div>
  )
}
