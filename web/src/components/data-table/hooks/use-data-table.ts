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
  type ColumnDef,
  type ColumnFiltersState,
  type ColumnSizingState,
  type ColumnVisibilityState,
  type ExpandedState,
  type OnChangeFn,
  type PaginationState,
  type ReactTable,
  type RowSelectionState,
  type RowData,
  type SortingState,
  type TableOptions,
  type Updater,
  columnFacetingFeature,
  columnFilteringFeature,
  columnResizingFeature,
  columnSizingFeature,
  columnVisibilityFeature,
  createExpandedRowModel,
  createFacetedRowModel,
  createFacetedUniqueValues,
  createFilteredRowModel,
  createPaginatedRowModel,
  createSortedRowModel,
  globalFilteringFeature,
  rowExpandingFeature,
  rowPaginationFeature,
  rowSelectionFeature,
  rowSortingFeature,
  tableFeatures,
  useTable,
} from '@tanstack/react-table'
import * as React from 'react'

import type { DataTableFeatures } from '../features'

type DataTableFeatureOptions<TData extends RowData> = Pick<
  TableOptions<DataTableFeatures, TData>,
  | 'enableRowSelection'
  | 'getRowId'
  | 'getSubRows'
  | 'globalFilterFn'
  | 'autoResetPageIndex'
  | 'manualFiltering'
  | 'manualPagination'
  | 'manualSorting'
  | 'enableSorting'
  | 'enableColumnResizing'
>

type DataTableStateOptions = {
  initialSorting?: SortingState
  sorting?: SortingState
  onSortingChange?: OnChangeFn<SortingState>
  initialColumnVisibility?: ColumnVisibilityState
  columnVisibilityStorageKey?: string | false
  columnVisibility?: ColumnVisibilityState
  onColumnVisibilityChange?: OnChangeFn<ColumnVisibilityState>
  initialColumnSizing?: ColumnSizingState
  columnSizingStorageKey?: string | false
  columnSizing?: ColumnSizingState
  onColumnSizingChange?: OnChangeFn<ColumnSizingState>
  initialRowSelection?: RowSelectionState
  rowSelection?: RowSelectionState
  onRowSelectionChange?: OnChangeFn<RowSelectionState>
  initialExpanded?: ExpandedState
  expanded?: ExpandedState
  onExpandedChange?: OnChangeFn<ExpandedState>
  columnFilters?: ColumnFiltersState
  onColumnFiltersChange?: OnChangeFn<ColumnFiltersState>
  globalFilter?: string
  onGlobalFilterChange?: OnChangeFn<string>
  initialPagination?: PaginationState
  pagination?: PaginationState
  onPaginationChange?: OnChangeFn<PaginationState>
}

type DataTableRowModelOptions = {
  withFilteredRowModel?: boolean
  withPaginationRowModel?: boolean
  withSortedRowModel?: boolean
  withFacetedRowModel?: boolean
  withExpandedRowModel?: boolean
}

type UseDataTableOptions<TData extends RowData> =
  DataTableFeatureOptions<TData> &
    DataTableStateOptions &
    DataTableRowModelOptions & {
      data: TData[]
      columns: ColumnDef<DataTableFeatures, TData, unknown>[]
      totalCount?: number
      pageCount?: number
      ensurePageInRange?: (pageCount: number) => void
    }

type ColumnSizingBounds = Record<
  string,
  {
    minSize?: number
    maxSize?: number
  }
>

type ColumnWithSizing<TData extends RowData> = ColumnDef<
  DataTableFeatures,
  TData,
  unknown
> & {
  accessorKey?: string | number
  columns?: ColumnDef<DataTableFeatures, TData, unknown>[]
}

const COLUMN_SIZING_PERSIST_DELAY_MS = 250

function resolveUpdater<TValue>(
  updater: Updater<TValue>,
  previous: TValue
): TValue {
  return typeof updater === 'function'
    ? (updater as (old: TValue) => TValue)(previous)
    : updater
}

function useControllableTableState<TValue>(
  controlledValue: TValue | undefined,
  defaultValue: TValue,
  onChange: OnChangeFn<TValue> | undefined
): [TValue, OnChangeFn<TValue>] {
  const [uncontrolledValue, setUncontrolledValue] =
    React.useState<TValue>(defaultValue)

  const value = controlledValue ?? uncontrolledValue

  const setValue = React.useCallback<OnChangeFn<TValue>>(
    (updater) => {
      if (controlledValue === undefined) {
        setUncontrolledValue((previous) => resolveUpdater(updater, previous))
      }
      onChange?.(updater)
    },
    [controlledValue, onChange]
  )

  return [value, setValue]
}

function readColumnVisibility(
  storageKey: string | undefined
): ColumnVisibilityState {
  if (!storageKey || typeof window === 'undefined') return {}

  try {
    const raw = window.localStorage.getItem(storageKey)
    if (!raw) return {}

    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {}
    }

    return Object.entries(parsed).reduce<ColumnVisibilityState>(
      (visibility, [key, value]) => {
        if (typeof value === 'boolean') {
          visibility[key] = value
        }
        return visibility
      },
      {}
    )
  } catch {
    return {}
  }
}

function getColumnId<TData extends RowData>(
  column: ColumnDef<DataTableFeatures, TData, unknown>
) {
  const columnWithSizing = column as ColumnWithSizing<TData>

  if (typeof columnWithSizing.id === 'string') {
    return columnWithSizing.id
  }

  if (typeof columnWithSizing.accessorKey === 'string') {
    return columnWithSizing.accessorKey.replaceAll('.', '_')
  }

  if (typeof columnWithSizing.accessorKey === 'number') {
    return String(columnWithSizing.accessorKey)
  }

  return undefined
}

function buildColumnSizingBounds<TData extends RowData>(
  columns: ColumnDef<DataTableFeatures, TData, unknown>[]
): ColumnSizingBounds {
  return columns.reduce<ColumnSizingBounds>((bounds, column) => {
    const columnWithSizing = column as ColumnWithSizing<TData>
    const columnId = getColumnId(column)

    if (columnId) {
      const minSize =
        typeof columnWithSizing.minSize === 'number' &&
        Number.isFinite(columnWithSizing.minSize)
          ? columnWithSizing.minSize
          : undefined
      const maxSize =
        typeof columnWithSizing.maxSize === 'number' &&
        Number.isFinite(columnWithSizing.maxSize)
          ? columnWithSizing.maxSize
          : undefined

      if (minSize !== undefined || maxSize !== undefined) {
        bounds[columnId] = { minSize, maxSize }
      }
    }

    if (Array.isArray(columnWithSizing.columns)) {
      Object.assign(bounds, buildColumnSizingBounds(columnWithSizing.columns))
    }

    return bounds
  }, {})
}

function getBoundedColumnSize(
  columnId: string,
  value: unknown,
  bounds: ColumnSizingBounds
) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return undefined
  }

  const columnBounds = bounds[columnId]
  let size = value

  if (columnBounds?.minSize !== undefined && size < columnBounds.minSize) {
    size = columnBounds.minSize
  }

  if (columnBounds?.maxSize !== undefined && size > columnBounds.maxSize) {
    size = columnBounds.maxSize
  }

  return size > 0 ? size : undefined
}

function readColumnSizing(
  storageKey: string | undefined,
  bounds: ColumnSizingBounds
): ColumnSizingState {
  if (!storageKey || typeof window === 'undefined') return {}

  try {
    const raw = window.localStorage.getItem(storageKey)
    if (!raw) return {}

    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {}
    }

    return Object.entries(parsed).reduce<ColumnSizingState>(
      (sizing, [key, value]) => {
        const boundedSize = getBoundedColumnSize(key, value, bounds)

        if (boundedSize !== undefined) {
          sizing[key] = boundedSize
        }
        return sizing
      },
      {}
    )
  } catch {
    return {}
  }
}

export function useDataTable<TData extends RowData>(
  options: UseDataTableOptions<TData>
) {
  const {
    data,
    columns,
    totalCount,
    pageCount: explicitPageCount,
    ensurePageInRange,
    manualFiltering,
    manualPagination,
    manualSorting,
    initialSorting = [],
    initialColumnVisibility = {},
    initialColumnSizing = {},
    initialRowSelection = {},
    initialExpanded = {},
    initialPagination = { pageIndex: 0, pageSize: 20 },
    withFilteredRowModel = !manualFiltering,
    withPaginationRowModel = !manualPagination,
    withSortedRowModel = !manualSorting && !manualPagination,
    withFacetedRowModel = !manualFiltering,
    withExpandedRowModel = false,
  } = options

  const columnVisibilityStorageKey =
    typeof options.columnVisibilityStorageKey === 'string'
      ? options.columnVisibilityStorageKey
      : undefined
  const columnSizingStorageKey =
    typeof options.columnSizingStorageKey === 'string'
      ? options.columnSizingStorageKey
      : undefined
  const resolvedInitialColumnVisibility = React.useMemo(
    () => ({
      ...initialColumnVisibility,
      ...readColumnVisibility(columnVisibilityStorageKey),
    }),
    [columnVisibilityStorageKey, initialColumnVisibility]
  )
  const columnSizingBounds = React.useMemo(
    () => buildColumnSizingBounds(columns),
    [columns]
  )
  const resolvedInitialColumnSizing = React.useMemo(
    () => ({
      ...initialColumnSizing,
      ...readColumnSizing(columnSizingStorageKey, columnSizingBounds),
    }),
    [columnSizingBounds, columnSizingStorageKey, initialColumnSizing]
  )

  const [sorting, onSortingChange] = useControllableTableState(
    options.sorting,
    initialSorting,
    options.onSortingChange
  )
  const [columnVisibility, onColumnVisibilityChange] =
    useControllableTableState(
      options.columnVisibility,
      resolvedInitialColumnVisibility,
      options.onColumnVisibilityChange
    )
  const [columnSizing, onColumnSizingChange] = useControllableTableState(
    options.columnSizing,
    resolvedInitialColumnSizing,
    options.onColumnSizingChange
  )
  const hydratedColumnVisibilityStorageKeyRef = React.useRef(
    columnVisibilityStorageKey
  )
  const hydratedColumnSizingStorageKeyRef = React.useRef(columnSizingStorageKey)
  const skipNextColumnVisibilityPersistRef = React.useRef(false)
  const skipNextColumnSizingPersistRef = React.useRef(false)
  const columnSizingPersistTimerRef = React.useRef<number | undefined>(
    undefined
  )
  const [rowSelection, onRowSelectionChange] = useControllableTableState(
    options.rowSelection,
    initialRowSelection,
    options.onRowSelectionChange
  )
  const [expanded, onExpandedChange] = useControllableTableState(
    options.expanded,
    initialExpanded,
    options.onExpandedChange
  )
  const [pagination, onPaginationChange] = useControllableTableState(
    options.pagination,
    initialPagination,
    options.onPaginationChange
  )

  const resolvedPageCount =
    explicitPageCount ??
    (totalCount !== undefined
      ? Math.ceil(totalCount / pagination.pageSize)
      : undefined)
  const resolvedEnableSorting =
    options.enableSorting ??
    (!manualPagination ||
      Boolean(options.sorting) ||
      Boolean(options.onSortingChange))

  // v9 registers row-model factories on the feature set. Register them
  // conditionally to preserve the v8 semantics the table options relied on:
  // sorting was disabled under manual pagination, and faceting/filtered row
  // models were skipped for manually filtered tables. The object is typed as
  // the shared DataTableFeatures so column definitions stay uniform.
  const features = React.useMemo(() => {
    const rowModels: Record<string, unknown> = {}
    if (withSortedRowModel) {
      rowModels.sortedRowModel = createSortedRowModel()
    }
    if (withFilteredRowModel) {
      rowModels.filteredRowModel = createFilteredRowModel()
    }
    if (withPaginationRowModel) {
      rowModels.paginatedRowModel = createPaginatedRowModel()
    }
    if (withFacetedRowModel) {
      rowModels.facetedRowModel = createFacetedRowModel()
      rowModels.facetedUniqueValues = createFacetedUniqueValues()
    }
    if (withExpandedRowModel) {
      rowModels.expandedRowModel = createExpandedRowModel()
    }
    return tableFeatures({
      rowSelectionFeature,
      rowSortingFeature,
      rowPaginationFeature,
      columnFilteringFeature,
      globalFilteringFeature,
      columnVisibilityFeature,
      columnSizingFeature,
      columnResizingFeature,
      rowExpandingFeature,
      columnFacetingFeature,
      ...rowModels,
    } as DataTableFeatures)
  }, [
    withSortedRowModel,
    withFilteredRowModel,
    withPaginationRowModel,
    withFacetedRowModel,
    withExpandedRowModel,
  ])

  // v9 syncs controlled state slices into the table store on every render
  // and reacts to their identity, where v8 tolerated fresh references.
  // Inline `columnFilters` arrays (for example `[]` literals) would
  // otherwise churn the store and remount the table rows, so reuse the
  // previous array reference whenever the serialized content is unchanged.
  const columnFiltersSignature = options.columnFilters
    ? JSON.stringify(options.columnFilters)
    : undefined
  // The signature (not the array identity) is the memo dependency: when the
  // content is unchanged the cached reference is reused.
  const stableColumnFilters = React.useMemo<ColumnFiltersState | undefined>(
    () => options.columnFilters,
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [columnFiltersSignature]
  )

  const table = useTable({
    features,
    data,
    columns,
    rowCount: totalCount,
    pageCount: resolvedPageCount,
    state: {
      sorting,
      columnVisibility,
      columnSizing,
      rowSelection,
      expanded,
      columnFilters: stableColumnFilters,
      globalFilter: options.globalFilter,
      pagination,
    },
    enableRowSelection: options.enableRowSelection,
    enableSorting: resolvedEnableSorting,
    getRowId: options.getRowId,
    getSubRows: options.getSubRows,
    globalFilterFn: options.globalFilterFn,
    autoResetPageIndex: options.autoResetPageIndex,
    manualFiltering,
    manualPagination,
    manualSorting,
    enableColumnResizing: options.enableColumnResizing,
    columnResizeMode: 'onChange',
    onSortingChange,
    onColumnVisibilityChange,
    onColumnSizingChange,
    onRowSelectionChange,
    onExpandedChange,
    onColumnFiltersChange: options.onColumnFiltersChange,
    onGlobalFilterChange: options.onGlobalFilterChange,
    onPaginationChange,
  })

  // v9 rebuilds the value returned by `useTable` whenever the options literal
  // or any state slice changes, so its identity differs on nearly every
  // render. v8 handed back one stable instance and the tables built on this
  // hook rely on that: `table` appears in effect dependency arrays, and an
  // effect that calls a `table.set*`/`reset*` method re-fires on each identity
  // change, which updates state, which changes the identity again.
  // Keep one handle per mount and sync it with the freshly built instance.
  const tableHandleRef = React.useRef<
    ReactTable<DataTableFeatures, TData> | undefined
  >(undefined)
  if (tableHandleRef.current === undefined) {
    tableHandleRef.current = { ...table }
  } else {
    Object.assign(tableHandleRef.current, table)
  }
  const stableTable = tableHandleRef.current

  const actualPageCount = stableTable.getPageCount()
  React.useEffect(() => {
    ensurePageInRange?.(actualPageCount)
  }, [actualPageCount, ensurePageInRange])

  React.useEffect(() => {
    if (
      options.columnVisibility !== undefined ||
      columnVisibilityStorageKey ===
        hydratedColumnVisibilityStorageKeyRef.current
    ) {
      return
    }

    hydratedColumnVisibilityStorageKeyRef.current = columnVisibilityStorageKey
    skipNextColumnVisibilityPersistRef.current = true
    onColumnVisibilityChange(() => resolvedInitialColumnVisibility)
  }, [
    columnVisibilityStorageKey,
    onColumnVisibilityChange,
    options.columnVisibility,
    resolvedInitialColumnVisibility,
  ])

  React.useEffect(() => {
    if (
      options.columnSizing !== undefined ||
      columnSizingStorageKey === hydratedColumnSizingStorageKeyRef.current
    ) {
      return
    }

    hydratedColumnSizingStorageKeyRef.current = columnSizingStorageKey
    skipNextColumnSizingPersistRef.current = true
    onColumnSizingChange(() => resolvedInitialColumnSizing)
  }, [
    columnSizingStorageKey,
    onColumnSizingChange,
    options.columnSizing,
    resolvedInitialColumnSizing,
  ])

  React.useEffect(() => {
    if (!columnVisibilityStorageKey || typeof window === 'undefined') return

    if (skipNextColumnVisibilityPersistRef.current) {
      skipNextColumnVisibilityPersistRef.current = false
      return
    }

    try {
      window.localStorage.setItem(
        columnVisibilityStorageKey,
        JSON.stringify(columnVisibility)
      )
    } catch {
      // Storage can be unavailable in private mode; table controls still work.
    }
  }, [columnVisibility, columnVisibilityStorageKey])

  React.useEffect(() => {
    if (!columnSizingStorageKey || typeof window === 'undefined') return

    if (skipNextColumnSizingPersistRef.current) {
      skipNextColumnSizingPersistRef.current = false
      return
    }

    if (columnSizingPersistTimerRef.current !== undefined) {
      window.clearTimeout(columnSizingPersistTimerRef.current)
    }

    columnSizingPersistTimerRef.current = window.setTimeout(() => {
      try {
        window.localStorage.setItem(
          columnSizingStorageKey,
          JSON.stringify(columnSizing)
        )
      } catch {
        // Storage can be unavailable in private mode; table controls still work.
      } finally {
        columnSizingPersistTimerRef.current = undefined
      }
    }, COLUMN_SIZING_PERSIST_DELAY_MS)

    return () => {
      if (columnSizingPersistTimerRef.current !== undefined) {
        window.clearTimeout(columnSizingPersistTimerRef.current)
        columnSizingPersistTimerRef.current = undefined
      }
    }
  }, [columnSizing, columnSizingStorageKey])

  return {
    table: stableTable,
  }
}
