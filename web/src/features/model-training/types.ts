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
export type FinetuneJob = {
  id: number
  user_id: number
  name: string
  base_model: string
  method: string
  dataset_refs: string
  hyper_params: string
  status: number
  result_model: string
  error_msg: string
  created_at: string
  updated_at: string
}

export type FinetuneJobFormData = {
  name: string
  base_model: string
  method: string
  dataset_refs: string
  hyper_params: string
}

export type AdminJobStatusPayload = {
  status: number
  result_model?: string
  error_msg?: string
}

export type ApiResponse<T = unknown> = {
  success: boolean
  message: string
  data: T
}

export type PagedData<T> = {
  page: number
  page_size: number
  total: number
  items: T[]
}

export type ListParams = {
  p?: number
  page_size?: number
  keyword?: string
  status?: number
  userId?: number
}
