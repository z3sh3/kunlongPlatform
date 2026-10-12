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

/** Mirrors the enterprise module's program types on the backend. */
export const PROGRAM_TYPE = {
  INCUBATION: 'incubation',
  SME_SUPPORT: 'sme_support',
  GOV_PARK: 'gov_park',
} as const

/** Mirrors modules/shared publish statuses on the backend. */
export const PUBLISH_STATUS = {
  DRAFT: 0,
  PUBLISHED: 1,
  OFFLINE: 2,
} as const

/** Mirrors modules/shared review statuses on the backend. */
export const REVIEW_STATUS = {
  PENDING: 0,
  REVIEWING: 1,
  APPROVED: 2,
  REJECTED: 3,
} as const

type Option = { value: number; labelKey: string }

export const PUBLISH_STATUS_OPTIONS: Option[] = [
  { value: PUBLISH_STATUS.DRAFT, labelKey: 'Draft' },
  { value: PUBLISH_STATUS.PUBLISHED, labelKey: 'Published' },
  { value: PUBLISH_STATUS.OFFLINE, labelKey: 'Offline' },
]

export const REVIEW_STATUS_OPTIONS: Option[] = [
  { value: REVIEW_STATUS.PENDING, labelKey: 'Pending' },
  { value: REVIEW_STATUS.REVIEWING, labelKey: 'Under review' },
  { value: REVIEW_STATUS.APPROVED, labelKey: 'Approved' },
  { value: REVIEW_STATUS.REJECTED, labelKey: 'Rejected' },
]

export function getOptionLabelKey(
  options: Option[],
  value: number
): string | undefined {
  return options.find((option) => option.value === value)?.labelKey
}
