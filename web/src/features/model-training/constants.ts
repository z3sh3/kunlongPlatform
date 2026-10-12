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

/** Mirrors modelsquare job statuses on the backend. */
export const JOB_STATUS = {
  DRAFT: 0,
  SUBMITTED: 1,
  QUEUED: 2,
  RUNNING: 3,
  SUCCEEDED: 4,
  FAILED: 5,
  CANCELED: 6,
} as const

export const FINETUNE_METHODS = ['lora', 'sft', 'full'] as const

type Option = { value: number; labelKey: string }

export const JOB_STATUS_OPTIONS: Option[] = [
  { value: JOB_STATUS.DRAFT, labelKey: 'Draft' },
  { value: JOB_STATUS.SUBMITTED, labelKey: 'Submitted' },
  { value: JOB_STATUS.QUEUED, labelKey: 'Queued' },
  { value: JOB_STATUS.RUNNING, labelKey: 'Running' },
  { value: JOB_STATUS.SUCCEEDED, labelKey: 'Succeeded' },
  { value: JOB_STATUS.FAILED, labelKey: 'Failed' },
  { value: JOB_STATUS.CANCELED, labelKey: 'Canceled' },
]

/** Statuses an administrator may set through the admin console. */
export const ADMIN_JOB_STATUS_OPTIONS: Option[] = [
  { value: JOB_STATUS.QUEUED, labelKey: 'Queued' },
  { value: JOB_STATUS.RUNNING, labelKey: 'Running' },
  { value: JOB_STATUS.SUCCEEDED, labelKey: 'Succeeded' },
  { value: JOB_STATUS.FAILED, labelKey: 'Failed' },
]

export function getOptionLabelKey(
  options: Option[],
  value: number
): string | undefined {
  return options.find((option) => option.value === value)?.labelKey
}
