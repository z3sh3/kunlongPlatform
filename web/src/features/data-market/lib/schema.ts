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

import { PRICE_MODE } from '../constants'
import type { DatasetFormData } from '../types'

export const datasetFormSchema = z
  .object({
    name: z.string().min(1, 'Name is required'),
    description: z.string(),
    category: z.string(),
    format: z.string(),
    file_url: z.string(),
    cover_url: z.string(),
    size_bytes: z.number().int().min(0),
    price_mode: z.number(),
    unit_price: z.number().int().min(0),
  })
  .refine(
    (values) =>
      values.price_mode !== PRICE_MODE.PAID || values.unit_price > 0,
    {
      path: ['unit_price'],
      message: 'Paid datasets require a positive unit price',
    }
  )

export type DatasetFormValues = z.infer<typeof datasetFormSchema>

export const DATASET_FORM_DEFAULT_VALUES: DatasetFormData = {
  name: '',
  description: '',
  category: '',
  format: '',
  file_url: '',
  cover_url: '',
  size_bytes: 0,
  price_mode: PRICE_MODE.FREE,
  unit_price: 0,
}
