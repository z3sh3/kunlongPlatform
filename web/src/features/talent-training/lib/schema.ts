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

import { COURSE_CATEGORY, PRICE_MODE } from '../constants'

export const courseFormSchema = z
  .object({
    title: z.string().min(1, 'Title is required'),
    category: z.enum([COURSE_CATEGORY.CERTIFICATION, COURSE_CATEGORY.CONSULTATION]),
    description: z.string(),
    content: z.string(),
    schedule: z.string(),
    capacity: z.number().int().min(0),
    price_mode: z.number(),
    price: z.number().int().min(0),
  })
  .refine((values) => values.price_mode !== PRICE_MODE.PAID || values.price > 0, {
    path: ['price'],
    message: 'Paid courses require a positive price',
  })

export type CourseFormValues = z.infer<typeof courseFormSchema>

export const COURSE_FORM_DEFAULT_VALUES: CourseFormValues = {
  title: '',
  category: COURSE_CATEGORY.CERTIFICATION,
  description: '',
  content: '',
  schedule: '',
  capacity: 0,
  price_mode: PRICE_MODE.FREE,
  price: 0,
}
