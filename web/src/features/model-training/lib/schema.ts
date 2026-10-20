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
import { z } from 'zod'

import { FINETUNE_METHODS } from '../constants'

export const LR_SCHEDULERS = [
  'cosine',
  'linear',
  'constant',
  'constant_with_warmup',
] as const

/**
 * Hyper-parameters are edited as structured fields but stored as a JSON object
 * string, which is the contract the backend keeps opaque and the training
 * service validates against its own framework limits.
 */
export const jobFormSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  base_model: z.string().min(1, 'Base model is required'),
  method: z.enum(FINETUNE_METHODS),
  dataset_id: z.number().int().positive('Select a dataset'),
  lora_rank: z.number().int().min(1).max(256),
  cutoff_len: z.number().int().min(128).max(32768),
  num_train_epochs: z.number().min(0.1).max(20),
  learning_rate: z.number().min(1e-7).max(1),
  per_device_train_batch_size: z.number().int().min(1).max(128),
  gradient_accumulation_steps: z.number().int().min(1).max(256),
  lr_scheduler_type: z.enum(LR_SCHEDULERS),
  bf16: z.boolean(),
})

export type JobFormValues = z.infer<typeof jobFormSchema>

export const JOB_FORM_DEFAULT_VALUES: JobFormValues = {
  name: '',
  base_model: '',
  method: 'lora',
  dataset_id: 0,
  lora_rank: 8,
  cutoff_len: 2048,
  num_train_epochs: 3,
  learning_rate: 0.0001,
  per_device_train_batch_size: 1,
  gradient_accumulation_steps: 8,
  lr_scheduler_type: 'cosine',
  bf16: true,
}

const HYPER_PARAM_KEYS = [
  'lora_rank',
  'cutoff_len',
  'num_train_epochs',
  'learning_rate',
  'per_device_train_batch_size',
  'gradient_accumulation_steps',
  'lr_scheduler_type',
  'bf16',
] as const

/** Serialises the structured hyper-parameters into the stored JSON object. */
export function toHyperParamsJSON(values: JobFormValues): string {
  const hyperParams: Record<string, unknown> = {}
  for (const key of HYPER_PARAM_KEYS) {
    hyperParams[key] = values[key]
  }
  return JSON.stringify(hyperParams)
}

/**
 * Reads stored hyper-parameters back into form values. Unknown or malformed
 * values fall back to the defaults instead of failing the edit dialog.
 */
export function parseHyperParamsJSON(raw: string): Partial<JobFormValues> {
  if (!raw) return {}
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return {}
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}

  const source = parsed as Record<string, unknown>
  const values: Partial<JobFormValues> = {}
  for (const key of HYPER_PARAM_KEYS) {
    const value = source[key]
    if (value === undefined || value === null) continue
    if (key === 'lr_scheduler_type') {
      if (typeof value === 'string' && LR_SCHEDULERS.includes(value as never)) {
        values.lr_scheduler_type = value as JobFormValues['lr_scheduler_type']
      }
      continue
    }
    if (key === 'bf16') {
      if (typeof value === 'boolean') values.bf16 = value
      continue
    }
    if (typeof value === 'number' && Number.isFinite(value)) {
      values[key] = value as never
    }
  }
  return values
}
