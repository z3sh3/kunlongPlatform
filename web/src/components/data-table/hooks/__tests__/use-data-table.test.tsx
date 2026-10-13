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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useEffect, useState } from 'react'
import { beforeEach, expect, it } from 'vitest'

import { useDataTable } from '../use-data-table'

interface Row {
  id: number
}

const rows: Row[] = [{ id: 1 }, { id: 2 }, { id: 3 }]

const columns = [{ accessorKey: 'id' }]

const tableConfig = {
  manualPagination: true,
  manualSorting: true,
  manualFiltering: true,
  enableRowSelection: true,
} as const

let identities: unknown[] = []

// Mirrors the channels table, which resets the row selection from an effect
// that lists `table` as a dependency. A handle that changes identity every
// render makes that effect re-fire every render, and the reset is a state
// update, so the component never settles. The options are passed as a fresh
// literal, the way every caller of this hook does.
function SelectionResetProbe() {
  const [batchMode] = useState(false)
  const { table } = useDataTable<Row>({
    data: rows,
    columns,
    ...tableConfig,
  })
  identities.push(table)

  useEffect(() => {
    if (!batchMode) {
      table.resetRowSelection()
    }
  }, [batchMode, table])

  return <div>done</div>
}

// The shared handle must also stay in sync, otherwise render reads such as
// `table.state` would freeze at the value captured on mount.
function SelectionProbe() {
  const { table } = useDataTable<Row>({ data: rows, columns, ...tableConfig })
  const selectedCount = Object.keys(table.state.rowSelection ?? {}).length

  return (
    <button type='button' onClick={() => table.setRowSelection({ 1: true })}>
      selected {selectedCount}
    </button>
  )
}

beforeEach(() => {
  identities = []
})

it('returns a table handle whose identity is stable across renders', () => {
  render(<SelectionResetProbe />)

  expect(identities.length).toBeLessThan(10)
  expect(new Set(identities).size).toBe(1)
})

it('reflects table state changes through the stable handle', async () => {
  const user = userEvent.setup()
  render(<SelectionProbe />)

  expect(screen.getByRole('button')).toHaveTextContent('selected 0')
  await user.click(screen.getByRole('button'))
  expect(screen.getByRole('button')).toHaveTextContent('selected 1')
})
