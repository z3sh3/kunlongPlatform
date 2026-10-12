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
import {
  ChevronLeft as ChevronLeftIcon,
  ChevronRight as ChevronRightIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const PAGE_SIZE_OPTIONS = [10, 20, 30, 40, 50, 100] as const
const PAGE_SIZE_SELECT_ITEMS = PAGE_SIZE_OPTIONS.map((pageSize) => ({
  value: `${pageSize}`,
  label: pageSize,
}))

type ServerPaginationProps = {
  /** 1-based page number of the current server-side page. */
  page: number
  pageSize: number
  /** Total row count reported by the server. */
  total: number
  onPageChange: (page: number) => void
  onPageSizeChange?: (pageSize: number) => void
  disabled?: boolean
}

/**
 * Pagination controls for lists whose pages come from the server instead
 * of a local TanStack table instance. Mirrors the visual contract of
 * DataTablePagination's compact mode.
 */
export function ServerPagination(props: ServerPaginationProps) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(props.total / props.pageSize))

  return (
    <nav
      aria-label={t('Page')}
      className='flex w-full min-w-0 flex-wrap items-center justify-between gap-2 text-sm'
    >
      <span className='text-muted-foreground min-w-0 [overflow-wrap:anywhere]'>
        {t('Total:')} {props.total.toLocaleString()}
      </span>
      <div className='flex items-center gap-2'>
        {props.onPageSizeChange ? (
          <Select
            items={PAGE_SIZE_SELECT_ITEMS}
            value={`${props.pageSize}`}
            onValueChange={(value) => props.onPageSizeChange?.(Number(value))}
          >
            <SelectTrigger
              className='text-foreground h-8 w-[64px] font-medium tabular-nums sm:w-[70px]'
              disabled={props.disabled}
            >
              <SelectValue placeholder={props.pageSize} />
            </SelectTrigger>
            <SelectContent side='top' alignItemWithTrigger={false}>
              <SelectGroup>
                {PAGE_SIZE_OPTIONS.map((pageSize) => (
                  <SelectItem key={pageSize} value={`${pageSize}`}>
                    {pageSize}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        ) : null}
        <span className='tabular-nums' aria-live='polite'>
          {props.page} / {totalPages}
        </span>
        <Button
          variant='outline'
          size='icon'
          className='size-11'
          aria-label={t('Go to previous page')}
          onClick={() => props.onPageChange(Math.max(1, props.page - 1))}
          disabled={props.disabled || props.page <= 1}
        >
          <ChevronLeftIcon />
        </Button>
        <Button
          variant='outline'
          size='icon'
          className='size-11'
          aria-label={t('Go to next page')}
          onClick={() => props.onPageChange(props.page + 1)}
          disabled={props.disabled || props.page >= totalPages}
        >
          <ChevronRightIcon />
        </Button>
      </div>
    </nav>
  )
}
