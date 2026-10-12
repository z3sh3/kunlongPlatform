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
import { toIntlLocale } from '@/i18n/languages'
import { useAuthStore } from '@/stores/auth-store'

import {
  adminListDatasets,
  adminUpdateDatasetStatus,
  applyDataset,
  deleteDataset,
  listDatasets,
} from '../api'
import {
  PRICE_MODE,
  PUBLISH_STATUS,
  getOptionLabelKey,
  PUBLISH_STATUS_OPTIONS,
} from '../constants'
import type { Dataset } from '../types'
import { DatasetFormDialog } from './dataset-form-dialog'

type ListMode = 'browse' | 'mine' | 'admin'

type DatasetsListProps = {
  mode: ListMode
}

export function DatasetsList(props: DatasetsListProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const queryClient = useQueryClient()
  const user = useAuthStore((s) => s.auth.user)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Dataset | null>(null)

  const isMine = props.mode === 'mine'
  const isAdmin = props.mode === 'admin'

  const datasetsQuery = useQuery({
    queryKey: ['data-market', 'datasets', props.mode, page, pageSize],
    queryFn: () =>
      isAdmin
        ? adminListDatasets({ p: page, page_size: pageSize })
        : listDatasets({
            p: page,
            page_size: pageSize,
            mine: isMine,
          }),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['data-market', 'datasets'] })
  }

  const applyMutation = useMutation({
    mutationFn: (id: number) => applyDataset(id),
    onSuccess: () => {
      toast.success(t('Application submitted'))
      invalidate()
      queryClient.invalidateQueries({ queryKey: ['data-market', 'grants'] })
    },
    onError: (error) => handleServerError(error),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteDataset(id),
    onSuccess: () => {
      toast.success(t('Dataset deleted'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const statusMutation = useMutation({
    mutationFn: (values: { id: number; status: number }) =>
      adminUpdateDatasetStatus(values.id, values.status),
    onSuccess: () => {
      toast.success(t('Dataset status updated'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const renderAdminStatusButton = (dataset: Dataset) => {
    const nextStatus =
      dataset.status === PUBLISH_STATUS.PUBLISHED
        ? PUBLISH_STATUS.OFFLINE
        : PUBLISH_STATUS.PUBLISHED
    const label =
      dataset.status === PUBLISH_STATUS.PUBLISHED
        ? t('Take offline')
        : t('Publish')
    return (
      <Button
        variant='ghost'
        size='sm'
        onClick={() =>
          statusMutation.mutate({ id: dataset.id, status: nextStatus })
        }
        disabled={statusMutation.isPending}
      >
        {label}
      </Button>
    )
  }

  const columns = [
    {
      id: 'name',
      header: t('Name'),
      cell: (dataset: Dataset) => (
        <div className='min-w-0'>
          <p className='truncate font-medium'>{dataset.name}</p>
          {dataset.description ? (
            <p className='text-muted-foreground truncate text-xs'>
              {dataset.description}
            </p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'category',
      header: t('Category'),
      cell: (dataset: Dataset) => dataset.category || '-',
    },
    ...(isAdmin
      ? [
          {
            id: 'status',
            header: t('Status'),
            cell: (dataset: Dataset) => (
              <Badge variant='outline'>
                {t(getOptionLabelKey(PUBLISH_STATUS_OPTIONS, dataset.status) ?? 'Draft')}
              </Badge>
            ),
          },
        ]
      : []),
    {
      id: 'price',
      header: t('Price'),
      cell: (dataset: Dataset) =>
        dataset.price_mode === PRICE_MODE.PAID
          ? `${dataset.unit_price.toLocaleString(locale)}`
          : t('Free'),
    },
    {
      id: 'created_at',
      header: t('Created'),
      cell: (dataset: Dataset) =>
        dataset.created_at
          ? dayjs(dataset.created_at).format('YYYY-MM-DD')
          : '-',
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cell: (dataset: Dataset) => (
        <div className='flex justify-end gap-1'>
          {props.mode === 'browse' ? (
            <Button
              size='sm'
              onClick={() => applyMutation.mutate(dataset.id)}
              disabled={
                applyMutation.isPending || dataset.user_id === user?.id
              }
            >
              {t('Apply for access')}
            </Button>
          ) : null}
          {isMine ? (
            <>
              <Button
                variant='ghost'
                size='sm'
                onClick={() => {
                  setEditing(dataset)
                  setFormOpen(true)
                }}
                disabled={dataset.status !== PUBLISH_STATUS.DRAFT}
              >
                {t('Edit')}
              </Button>
              <Button
                variant='ghost'
                size='sm'
                className='text-destructive hover:text-destructive'
                onClick={() => deleteMutation.mutate(dataset.id)}
                disabled={
                  deleteMutation.isPending ||
                  dataset.status === PUBLISH_STATUS.PUBLISHED
                }
              >
                {t('Delete')}
              </Button>
            </>
          ) : null}
          {isAdmin ? renderAdminStatusButton(dataset) : null}
        </div>
      ),
    },
  ]

  const items = datasetsQuery.data?.items ?? []

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
            {t('Create dataset')}
          </Button>
        </div>
      ) : null}
      <StaticDataTable
        columns={columns}
        data={items}
        getRowKey={(dataset) => dataset.id}
        empty={datasetsQuery.isSuccess && items.length === 0}
        emptyContent={t('No datasets yet')}
      />
      {datasetsQuery.data ? (
        <ServerPagination
          page={page}
          pageSize={pageSize}
          total={datasetsQuery.data.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          disabled={datasetsQuery.isFetching}
        />
      ) : null}
      <DatasetFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        dataset={editing}
      />
    </div>
  )
}
