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

/** Dataset intake statuses. */
export const DATASET_STATUS = {
  PENDING: 0,
  READY: 1,
  FAILED: 2,
} as const

/** Dataset intake sources; a prepared dataset is produced by DataFlow. */
export const DATASET_SOURCE = {
  UPLOAD: 'upload',
  URL: 'url',
  MARKET: 'market',
  PIPELINE: 'pipeline',
} as const

export const FINETUNE_METHODS = ['lora', 'sft', 'full'] as const

/** Methods that still have running or pending work. */
export const ACTIVE_JOB_STATUSES: number[] = [
  JOB_STATUS.SUBMITTED,
  JOB_STATUS.QUEUED,
  JOB_STATUS.RUNNING,
]

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

export const DATASET_SOURCE_OPTIONS: Array<{ value: string; labelKey: string }> = [
  { value: DATASET_SOURCE.UPLOAD, labelKey: 'Upload' },
  { value: DATASET_SOURCE.URL, labelKey: 'URL' },
  { value: DATASET_SOURCE.MARKET, labelKey: 'Data market' },
  { value: DATASET_SOURCE.PIPELINE, labelKey: 'DataFlow pipeline' },
]

export const FINETUNE_METHOD_OPTIONS: Array<{ value: string; labelKey: string }> = [
  { value: 'lora', labelKey: 'LoRA' },
  { value: 'sft', labelKey: 'Freeze tuning' },
  { value: 'full', labelKey: 'Full parameter' },
]

/** Stable error codes returned by the backend for service failures. */
export const TRAINING_ERROR_CODES = {
  NOT_CONFIGURED: 'training_service_not_configured',
  UNREACHABLE: 'training_service_unreachable',
  AUTH_FAILED: 'training_service_auth_failed',
} as const

/** Upload limit enforced by the gateway for a single dataset file. */
export const MAX_DATASET_UPLOAD_BYTES = 200 * 1024 * 1024

export function getOptionLabelKey<T extends number | string>(
  options: Array<{ value: T; labelKey: string }>,
  value: T
): string | undefined {
  return options.find((option) => option.value === value)?.labelKey
}

export function isActiveJobStatus(status: number): boolean {
  return ACTIVE_JOB_STATUSES.includes(status)
}
