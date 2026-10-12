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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ServerPagination, StaticDataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'

import { listMyGrants } from '../api'
import { getOptionLabelKey, REVIEW_STATUS_OPTIONS } from '../constants'

export function GrantsList() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  const grantsQuery = useQuery({
    queryKey: ['data-market', 'grants', page, pageSize],
    queryFn: () => listMyGrants({ p: page, page_size: pageSize }),
  })

  const items = grantsQuery.data?.items ?? []

  const columns = [
    {
      id: 'dataset',
      header: t('Dataset'),
      cell: (grant: (typeof items)[number]) => grant.dataset?.name ?? '-',
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (grant: (typeof items)[number]) => (
        <Badge variant='outline'>
          {t(
            getOptionLabelKey(REVIEW_STATUS_OPTIONS, grant.status) ?? 'Pending'
          )}
        </Badge>
      ),
    },
    {
      id: 'expires_at',
      header: t('Expires'),
      cell: (grant: (typeof items)[number]) =>
        grant.expires_at > 0
          ? dayjs.unix(grant.expires_at).format('YYYY-MM-DD')
          : t('Never'),
    },
    {
      id: 'remark',
      header: t('Remark'),
      cell: (grant: (typeof items)[number]) => grant.remark || '-',
    },
    {
      id: 'created_at',
      header: t('Applied'),
      cell: (grant: (typeof items)[number]) =>
        grant.created_at ? dayjs(grant.created_at).format('YYYY-MM-DD') : '-',
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
