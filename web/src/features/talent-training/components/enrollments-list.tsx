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

import { adminListEnrollments, adminReviewEnrollment, listMyEnrollments } from '../api'
import {
  ENROLLMENT_STATUS,
  ENROLLMENT_STATUS_OPTIONS,
  REVIEW_STATUS,
  getOptionLabelKey,
} from '../constants'
import type { TrainingEnrollment } from '../types'

type EnrollmentsListMode = 'mine' | 'admin'

type EnrollmentsListProps = {
  mode: EnrollmentsListMode
}

export function EnrollmentsList(props: EnrollmentsListProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  const isAdmin = props.mode === 'admin'

  const enrollmentsQuery = useQuery({
    queryKey: [
      'talent-training',
      'enrollments',
      props.mode,
      page,
      pageSize,
    ],
    queryFn: () =>
      isAdmin
        ? adminListEnrollments({ p: page, page_size: pageSize })
        : listMyEnrollments({ p: page, page_size: pageSize }),
  })

  const reviewMutation = useMutation({
    mutationFn: (values: { id: number; status: number }) =>
      adminReviewEnrollment(values.id, values.status),
    onSuccess: () => {
      toast.success(t('Enrollment review saved'))
      queryClient.invalidateQueries({
        queryKey: ['talent-training', 'enrollments'],
      })
    },
    onError: (error) => handleServerError(error),
  })

  const renderAdminActions = (enrollment: TrainingEnrollment) => {
    const actions = []
    if (enrollment.status === REVIEW_STATUS.PENDING) {
      actions.push(
        <Button
          key='approve'
          variant='ghost'
          size='sm'
          onClick={() =>
            reviewMutation.mutate({
              id: enrollment.id,
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
              id: enrollment.id,
              status: REVIEW_STATUS.REJECTED,
            })
          }
          disabled={reviewMutation.isPending}
        >
          {t('Reject')}
        </Button>
      )
    }
    if (enrollment.status === REVIEW_STATUS.APPROVED) {
      actions.push(
        <Button
          key='complete'
          variant='ghost'
          size='sm'
          onClick={() =>
            reviewMutation.mutate({
              id: enrollment.id,
              status: ENROLLMENT_STATUS.COMPLETED,
            })
          }
          disabled={reviewMutation.isPending}
        >
          {t('Mark completed')}
        </Button>
      )
    }
    return actions
  }

  const columns = [
    {
      id: 'course',
      header: t('Course'),
      cell: (enrollment: TrainingEnrollment) =>
        enrollment.course?.title ?? '-',
    },
    ...(isAdmin
      ? [
          {
            id: 'user',
            header: t('User ID'),
            cell: (enrollment: TrainingEnrollment) => enrollment.user_id,
          },
        ]
      : []),
    {
      id: 'status',
      header: t('Status'),
      cell: (enrollment: TrainingEnrollment) => (
        <Badge variant='outline'>
          {t(
            getOptionLabelKey(ENROLLMENT_STATUS_OPTIONS, enrollment.status) ??
              'Pending'
          )}
        </Badge>
      ),
    },
    {
      id: 'remark',
      header: t('Remark'),
      cell: (enrollment: TrainingEnrollment) => enrollment.remark || '-',
    },
    {
      id: 'created_at',
      header: t('Applied'),
      cell: (enrollment: TrainingEnrollment) =>
        enrollment.created_at
          ? dayjs(enrollment.created_at).format('YYYY-MM-DD')
          : '-',
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cell: (enrollment: TrainingEnrollment) => (
        <div className='flex justify-end gap-1'>
          {isAdmin ? renderAdminActions(enrollment) : null}
        </div>
      ),
    },
  ]

  const items = enrollmentsQuery.data?.items ?? []

  return (
    <div className='space-y-4'>
      <StaticDataTable
        columns={columns}
        data={items}
        getRowKey={(enrollment) => enrollment.id}
        empty={enrollmentsQuery.isSuccess && items.length === 0}
        emptyContent={t('No enrollments yet')}
      />
      {enrollmentsQuery.data ? (
        <ServerPagination
          page={page}
          pageSize={pageSize}
          total={enrollmentsQuery.data.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          disabled={enrollmentsQuery.isFetching}
        />
      ) : null}
    </div>
  )
}
