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
import { toast } from 'sonner'

import { handleServerError } from '@/lib/handle-server-error'

import { TRAINING_ERROR_CODES } from '../constants'

/**
 * A business failure returned by the console API. Unlike an HTTP error it
 * carries the stable `code` the backend assigns to training service failures.
 */
export class TrainingServiceError extends Error {
  readonly code?: string

  constructor(message: string, code?: string) {
    super(message)
    this.name = 'TrainingServiceError'
    this.code = code
  }
}

/** Reports any failure from this feature with a single toast. */
export function showTrainingError(error: unknown): void {
  if (error instanceof TrainingServiceError) {
    toast.error(error.message)
    return
  }
  handleServerError(error)
}

/** True when the training service is not configured or cannot be reached. */
export function isTrainingServiceUnavailable(error: unknown): boolean {
  if (!(error instanceof TrainingServiceError)) return false
  return (
    error.code === TRAINING_ERROR_CODES.NOT_CONFIGURED ||
    error.code === TRAINING_ERROR_CODES.UNREACHABLE
  )
}
