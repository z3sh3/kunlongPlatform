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
along with this program.  If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { columnFacetingFeature, columnFilteringFeature, columnResizingFeature, columnSizingFeature, columnVisibilityFeature, createExpandedRowModel, createFacetedRowModel, createFacetedUniqueValues, createFilteredRowModel, createPaginatedRowModel, createSortedRowModel, globalFilteringFeature, rowExpandingFeature, rowPaginationFeature, rowSelectionFeature, rowSortingFeature, tableFeatures } from '@tanstack/react-table'

/**
 * Feature set shared by all tables built through `useDataTable`.
 *
 * The runtime object registered per table may skip row-model factories that
 * the v8-style options kept conditional (for example sorting under manual
 * pagination), so the value below is only the reference for the static
 * `DataTableFeatures` type used by column definitions.
 */
export const dataTableFeatures = tableFeatures({
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
  sortedRowModel: createSortedRowModel(),
  filteredRowModel: createFilteredRowModel(),
  paginatedRowModel: createPaginatedRowModel(),
  facetedRowModel: createFacetedRowModel(),
  facetedUniqueValues: createFacetedUniqueValues(),
  expandedRowModel: createExpandedRowModel(),
})

export type DataTableFeatures = typeof dataTableFeatures
