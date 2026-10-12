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

import { adminListGrants, adminReviewGrant } from '../api'
import { getOptionLabelKey, REVIEW_STATUS, REVIEW_STATUS_OPTIONS } from '../constants'
import type { DatasetGrant } from '../types'

export function AdminGrantsList() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  const grantsQuery = useQuery({
    queryKey: ['data-market', 'admin-grants', page, pageSize],
    queryFn: () => adminListGrants({ p: page, page_size: pageSize }),
  })

  const reviewMutation = useMutation({
    mutationFn: (values: { id: number; status: number }) =>
      adminReviewGrant(values.id, values.status),
    onSuccess: () => {
      toast.success(t('Grant review saved'))
      queryClient.invalidateQueries({
        queryKey: ['data-market', 'admin-grants'],
      })
    },
    onError: (error) => handleServerError(error),
  })

  const items = grantsQuery.data?.items ?? []

  const columns = [
    {
      id: 'dataset',
      header: t('Dataset'),
      cell: (grant: DatasetGrant) => grant.dataset?.name ?? '-',
    },
    {
      id: 'user',
      header: t('User ID'),
      cell: (grant: DatasetGrant) => grant.user_id,
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (grant: DatasetGrant) => (
        <Badge variant='outline'>
          {t(
            getOptionLabelKey(REVIEW_STATUS_OPTIONS, grant.status) ?? 'Pending'
          )}
        </Badge>
      ),
    },
    {
      id: 'created_at',
      header: t('Applied'),
      cell: (grant: DatasetGrant) =>
        grant.created_at ? dayjs(grant.created_at).format('YYYY-MM-DD') : '-',
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cell: (grant: DatasetGrant) => (
        <div className='flex justify-end gap-1'>
          {grant.status === REVIEW_STATUS.PENDING ? (
            <>
              <Button
                variant='ghost'
                size='sm'
                onClick={() =>
                  reviewMutation.mutate({
                    id: grant.id,
                    status: REVIEW_STATUS.APPROVED,
                  })
                }
                disabled={reviewMutation.isPending}
              >
                {t('Approve')}
              </Button>
              <Button
                variant='ghost'
                size='sm'
                className='text-destructive hover:text-destructive'
                onClick={() =>
                  reviewMutation.mutate({
                    id: grant.id,
                    status: REVIEW_STATUS.REJECTED,
                  })
                }
                disabled={reviewMutation.isPending}
              >
                {t('Reject')}
              </Button>
            </>
          ) : null}
        </div>
      ),
    },
  ]

  return (
    <div className='space-y-4'>
      <StaticDataTable
        columns={columns}
        data={items}
        getRowKey={(grant) => grant.id}
        empty={grantsQuery.isSuccess && items.length === 0}
        emptyContent={t('No access applications yet')}
      />
      {grantsQuery.data ? (
        <ServerPagination
          page={page}
          pageSize={pageSize}
          total={grantsQuery.data.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          disabled={grantsQuery.isFetching}
        />
      ) : null}
    </div>
  )
}
