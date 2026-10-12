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

export const jobFormSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  base_model: z.string().min(1, 'Base model is required'),
  method: z.enum(FINETUNE_METHODS),
  dataset_refs: z.string(),
  hyper_params: z.string(),
})

export type JobFormValues = z.infer<typeof jobFormSchema>

export const JOB_FORM_DEFAULT_VALUES: JobFormValues = {
  name: '',
  base_model: '',
  method: 'lora',
  dataset_refs: '',
  hyper_params: '',
}
