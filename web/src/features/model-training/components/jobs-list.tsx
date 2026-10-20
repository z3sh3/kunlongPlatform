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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { handleServerError } from '@/lib/handle-server-error'

import {
  adminListJobs,
  adminUpdateJobStatus,
  cancelJob,
  deleteJob,
  listJobs,
  submitJob,
} from '../api'
import {
  ADMIN_JOB_STATUS_OPTIONS,
  JOB_STATUS,
  JOB_STATUS_OPTIONS,
  getOptionLabelKey,
} from '../constants'
import type { FinetuneJob } from '../types'
import { JobDetailDrawer } from './job-detail-drawer'
import { JobFormDialog } from './job-form-dialog'

type JobsListMode = 'mine' | 'admin'

type JobsListProps = {
  mode: JobsListMode
}

export function JobsList(props: JobsListProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<FinetuneJob | null>(null)
  const [detail, setDetail] = useState<FinetuneJob | null>(null)
  const [statusDraft, setStatusDraft] = useState<{
    id: number
    status: number
  } | null>(null)

  const isAdmin = props.mode === 'admin'

  const jobsQuery = useQuery({
    queryKey: ['model-training', 'jobs', props.mode, page, pageSize],
    queryFn: () =>
      isAdmin
        ? adminListJobs({ p: page, page_size: pageSize })
        : listJobs({ p: page, page_size: pageSize }),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['model-training', 'jobs'] })
  }

  const submitMutation = useMutation({
    mutationFn: (id: number) => submitJob(id),
    onSuccess: () => {
      toast.success(t('Fine-tuning job submitted'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const cancelMutation = useMutation({
    mutationFn: (id: number) => cancelJob(id),
    onSuccess: () => {
      toast.success(t('Fine-tuning job canceled'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteJob(id),
    onSuccess: () => {
      toast.success(t('Fine-tuning job deleted'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const statusMutation = useMutation({
    mutationFn: (values: { id: number; status: number }) =>
      adminUpdateJobStatus(values.id, { status: values.status }),
    onSuccess: () => {
      toast.success(t('Job status updated'))
      setStatusDraft(null)
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const renderUserActions = (job: FinetuneJob) => {
    const actions = []
    if (job.status === JOB_STATUS.DRAFT) {
      actions.push(
        <Button
          key='edit'
          variant='ghost'
          size='sm'
          onClick={() => {
            setEditing(job)
            setFormOpen(true)
          }}
        >
          {t('Edit')}
        </Button>,
        <Button
          key='submit'
          variant='ghost'
          size='sm'
          onClick={() => submitMutation.mutate(job.id)}
          disabled={submitMutation.isPending}
        >
          {t('Submit')}
        </Button>
      )
    }
    if (
      job.status === JOB_STATUS.SUBMITTED ||
      job.status === JOB_STATUS.QUEUED ||
      job.status === JOB_STATUS.RUNNING
    ) {
      actions.push(
        <Button
          key='cancel'
          variant='ghost'
          size='sm'
          onClick={() => cancelMutation.mutate(job.id)}
          disabled={cancelMutation.isPending}
        >
          {t('Cancel')}
        </Button>
      )
    }
    if (
      job.status === JOB_STATUS.DRAFT ||
      job.status === JOB_STATUS.CANCELED ||
      job.status === JOB_STATUS.FAILED
    ) {
      actions.push(
        <Button
          key='delete'
          variant='ghost'
          size='sm'
          className='text-destructive hover:text-destructive'
          onClick={() => deleteMutation.mutate(job.id)}
          disabled={deleteMutation.isPending}
        >
          {t('Delete')}
        </Button>
      )
    }
    return actions
  }

  const columns = [
    {
      id: 'name',
      header: t('Name'),
      cell: (job: FinetuneJob) => (
        <div className='min-w-0'>
          <p className='truncate font-medium'>{job.name}</p>
          <p className='text-muted-foreground truncate text-xs'>
            {job.base_model} · {job.method}
          </p>
        </div>
      ),
    },
    ...(isAdmin
      ? [
          {
            id: 'user',
            header: t('User ID'),
            cell: (job: FinetuneJob) => job.user_id,
          },
        ]
      : []),
    {
      id: 'status',
      header: t('Status'),
      cell: (job: FinetuneJob) => (
        <Badge variant='outline'>
          {t(getOptionLabelKey(JOB_STATUS_OPTIONS, job.status) ?? 'Draft')}
        </Badge>
      ),
    },
    {
      id: 'result',
      header: t('Result'),
      cell: (job: FinetuneJob) =>
        job.result_model || (job.error_msg ? job.error_msg : '-'),
    },
    {
      id: 'progress',
      header: t('Progress'),
      cell: (job: FinetuneJob) => (job.progress > 0 ? `${job.progress}%` : '-'),
    },
    {
      id: 'created_at',
      header: t('Created'),
      cell: (job: FinetuneJob) =>
        job.created_at ? dayjs(job.created_at).format('YYYY-MM-DD') : '-',
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cell: (job: FinetuneJob) => (
        <div className='flex justify-end gap-1'>
          <Button variant='ghost' size='sm' onClick={() => setDetail(job)}>
            {t('Details')}
          </Button>
          {isAdmin ? (
            <div className='flex items-center justify-end gap-1'>
              <Select
                value={statusDraft?.id === job.id ? `${statusDraft.status}` : undefined}
                onValueChange={(value) =>
                  setStatusDraft({ id: job.id, status: Number(value) })
                }
                items={ADMIN_JOB_STATUS_OPTIONS.map((option) => ({
                  value: `${option.value}`,
                  label: option.labelKey,
                }))}
              >
                <SelectTrigger className='h-8 w-[110px]'>
                  <SelectValue placeholder={t('Update status')} />
                </SelectTrigger>
                <SelectContent>
                  {ADMIN_JOB_STATUS_OPTIONS.map((option) => (
                    <SelectItem
                      key={option.value}
                      value={`${option.value}`}
                    >
                      {t(option.labelKey)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                size='sm'
                variant='outline'
                disabled={
                  statusMutation.isPending || statusDraft?.id !== job.id
                }
                onClick={() =>
                  statusDraft &&
                  statusMutation.mutate({
                    id: job.id,
                    status: statusDraft.status,
                  })
                }
              >
                {t('Save')}
              </Button>
            </div>
          ) : (
            renderUserActions(job)
          )}
        </div>
      ),
    },
  ]

  const items = jobsQuery.data?.items ?? []

  return (
    <div className='space-y-4'>
      {!isAdmin ? (
        <div className='flex justify-end'>
          <Button
            onClick={() => {
              setEditing(null)
              setFormOpen(true)
            }}
          >
            <Plus />
            {t('Create fine-tuning job')}
          </Button>
        </div>
      ) : null}
      <StaticDataTable
        columns={columns}
        data={items}
        getRowKey={(job) => job.id}
        empty={jobsQuery.isSuccess && items.length === 0}
        emptyContent={t('No fine-tuning jobs yet')}
      />
      {jobsQuery.data ? (
        <ServerPagination
          page={page}
          pageSize={pageSize}
          total={jobsQuery.data.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          disabled={jobsQuery.isFetching}
        />
      ) : null}
      <JobFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        job={editing}
      />
      <JobDetailDrawer
        job={detail}
        open={detail !== null}
        onOpenChange={(open) => {
          if (!open) setDetail(null)
        }}
      />
    </div>
  )
}
