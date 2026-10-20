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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import { previewDataset } from '../api'
import type { Dataset } from '../types'

/**
 * Opens a dialog with the real rows of a dataset. The query only runs while the
 * dialog is open, so browsing a list never downloads dataset content.
 */
export function DatasetPreviewButton(props: { dataset: Dataset }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  const previewQuery = useQuery({
    queryKey: ['data-market', 'preview', props.dataset.id],
    queryFn: () => previewDataset(props.dataset.id),
    enabled: open,
  })

  const preview = previewQuery.data
  const columns = (preview?.columns ?? []).map((column) => ({
    id: column,
    header: column,
    cell: (row: Record<string, unknown>) => (
      <span className='block max-w-[280px] truncate text-xs'>
        {String(row[column] ?? '')}
      </span>
    ),
  }))
  // Rows have no identifier of their own; the position is stable for a preview.
  const rows = (preview?.rows ?? []).map((row, position) => ({
    ...row,
    previewPosition: position,
  }))

  return (
    <>
      <Button variant='ghost' size='sm' onClick={() => setOpen(true)}>
        {t('View data')}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className='max-h-[85vh] overflow-hidden sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle>{props.dataset.name}</DialogTitle>
            <DialogDescription>
              {preview?.available
                ? t('Showing {{shown}} of {{total}} rows', {
                    shown: preview.rows.length,
                    total: preview.total_rows,
                  })
                : t('Preview is not available for this dataset')}
            </DialogDescription>
          </DialogHeader>
          {preview?.available ? (
            <div className='max-h-[60vh] overflow-auto'>
              <StaticDataTable
                columns={columns}
                data={rows}
                getRowKey={(row) => String(row.previewPosition)}
              />
            </div>
          ) : (
            <p className='text-muted-foreground text-sm'>
              {preview?.reason ?? ''}
            </p>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
