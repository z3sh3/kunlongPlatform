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

import { PROGRAM_TYPE } from '../constants'

export const programFormSchema = z.object({
  title: z.string().min(1, 'Title is required'),
  type: z.enum([
    PROGRAM_TYPE.INCUBATION,
    PROGRAM_TYPE.SME_SUPPORT,
    PROGRAM_TYPE.GOV_PARK,
  ]),
  summary: z.string(),
  cover_url: z.string(),
  description: z.string(),
  requirements: z.string(),
})

export type ProgramFormValues = z.infer<typeof programFormSchema>

export const PROGRAM_FORM_DEFAULT_VALUES: ProgramFormValues = {
  title: '',
  type: PROGRAM_TYPE.INCUBATION,
  summary: '',
  cover_url: '',
  description: '',
  requirements: '',
}

export const applicationFormSchema = z.object({
  program_id: z.number().int().positive('Program is required'),
  company_name: z.string().min(1, 'Company name is required'),
  contact_name: z.string().min(1, 'Contact name is required'),
  contact_phone: z.string().min(1, 'Contact phone is required'),
  description: z.string().min(1, 'Description is required'),
})

export type ApplicationFormValues = z.infer<typeof applicationFormSchema>

export const APPLICATION_FORM_DEFAULT_VALUES: ApplicationFormValues = {
  program_id: 0,
  company_name: '',
  contact_name: '',
  contact_phone: '',
  description: '',
}
