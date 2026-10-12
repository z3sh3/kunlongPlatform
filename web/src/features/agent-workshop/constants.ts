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

/** Mirrors agentworkshop statuses on the backend. */
export const AGENT_STATUS = {
  DRAFT: 0,
  REVIEWING: 1,
  PUBLISHED: 2,
  OFFLINE: 3,
} as const

/** Mirrors modules/shared price modes on the backend. */
export const PRICE_MODE = {
  FREE: 0,
  PAID: 1,
} as const

type Option = { value: number; labelKey: string }

export const AGENT_STATUS_OPTIONS: Option[] = [
  { value: AGENT_STATUS.DRAFT, labelKey: 'Draft' },
  { value: AGENT_STATUS.REVIEWING, labelKey: 'Under review' },
  { value: AGENT_STATUS.PUBLISHED, labelKey: 'Published' },
  { value: AGENT_STATUS.OFFLINE, labelKey: 'Offline' },
]

export function getOptionLabelKey(
  options: Option[],
  value: number
): string | undefined {
  return options.find((option) => option.value === value)?.labelKey
}
