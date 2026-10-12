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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ServerPagination, StaticDataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { handleServerError } from '@/lib/handle-server-error'

import { adminListApplications, adminReviewApplication, listMyApplications } from '../api'
import { REVIEW_STATUS, REVIEW_STATUS_OPTIONS, getOptionLabelKey } from '../constants'
import type { IncubationApplication } from '../types'

type ApplicationsListMode = 'mine' | 'admin'

type ApplicationsListProps = {
  mode: ApplicationsListMode
}

export function ApplicationsList(props: ApplicationsListProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  const isAdmin = props.mode === 'admin'

  const applicationsQuery = useQuery({
    queryKey: [
      'enterprise-incubation',
      'applications',
      props.mode,
      page,
      pageSize,
    ],
    queryFn: () =>
      isAdmin
        ? adminListApplications({ p: page, page_size: pageSize })
        : listMyApplications({ p: page, page_size: pageSize }),
  })

  const reviewMutation = useMutation({
    mutationFn: (values: { id: number; status: number }) =>
      adminReviewApplication(values.id, values.status),
    onSuccess: () => {
      toast.success(t('Application review saved'))
      queryClient.invalidateQueries({
        queryKey: ['enterprise-incubation', 'applications'],
      })
    },
    onError: (error) => handleServerError(error),
  })

  const renderAdminActions = (application: IncubationApplication) => {
    const actions = []
    if (
      application.status === REVIEW_STATUS.PENDING ||
      application.status === REVIEW_STATUS.REVIEWING
    ) {
      if (application.status === REVIEW_STATUS.PENDING) {
        actions.push(
          <Button
            key='reviewing'
            variant='ghost'
            size='sm'
            onClick={() =>
              reviewMutation.mutate({
                id: application.id,
                status: REVIEW_STATUS.REVIEWING,
              })
            }
            disabled={reviewMutation.isPending}
          >
            {t('Start review')}
          </Button>
        )
      }
      actions.push(
        <Button
          key='approve'
          variant='ghost'
          size='sm'
          onClick={() =>
            reviewMutation.mutate({
              id: application.id,
              status: REVIEW_STATUS.APPROVED,
            })
          }
          disabled={reviewMutation.isPending}
        >
          {t('Approve')}
        </Button>,
        <Button
          key='reject'
          variant='ghost'
          size='sm'
          className='text-destructive hover:text-destructive'
          onClick={() =>
            reviewMutation.mutate({
              id: application.id,
              status: REVIEW_STATUS.REJECTED,
            })
          }
          disabled={reviewMutation.isPending}
        >
          {t('Reject')}
        </Button>
      )
    }
    return actions
  }

  const columns = [
    {
      id: 'company',
      header: t('Company'),
      cell: (application: IncubationApplication) => (
        <div className='min-w-0'>
          <p className='truncate font-medium'>{application.company_name}</p>
          <p className='text-muted-foreground truncate text-xs'>
            {application.contact_name} · {application.contact_phone}
          </p>
        </div>
      ),
    },
    {
      id: 'program',
      header: t('Program'),
      cell: (application: IncubationApplication) =>
        application.program?.title ?? '-',
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (application: IncubationApplication) => (
        <div className='flex flex-col gap-1'>
          <Badge variant='outline'>
            {t(
              getOptionLabelKey(REVIEW_STATUS_OPTIONS, application.status) ??
                'Pending'
            )}
          </Badge>
          {application.review_comment ? (
            <span className='text-muted-foreground max-w-[200px] truncate text-xs'>
              {application.review_comment}
            </span>
          ) : null}
        </div>
      ),
    },
    {
      id: 'created_at',
      header: t('Applied'),
      cell: (application: IncubationApplication) =>
        application.created_at
          ? dayjs(application.created_at).format('YYYY-MM-DD')
          : '-',
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cell: (application: IncubationApplication) => (
        <div className='flex justify-end gap-1'>
          {isAdmin ? renderAdminActions(application) : null}
        </div>
      ),
    },
  ]

  const items = applicationsQuery.data?.items ?? []

  return (
    <div className='space-y-4'>
      <StaticDataTable
        columns={columns}
        data={items}
        getRowKey={(application) => application.id}
        empty={applicationsQuery.isSuccess && items.length === 0}
        emptyContent={t('No applications yet')}
      />
      {applicationsQuery.data ? (
        <ServerPagination
          page={page}
          pageSize={pageSize}
          total={applicationsQuery.data.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          disabled={applicationsQuery.isFetching}
        />
      ) : null}
    </div>
  )
}
