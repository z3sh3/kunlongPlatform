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
export type Agent = {
  id: number
  user_id: number
  name: string
  description: string
  icon: string
  category: string
  config: string
  price_mode: number
  unit_price: number
  status: number
  review_comment: string
  created_at: string
  updated_at: string
}

export type AgentFormData = {
  name: string
  description: string
  icon: string
  category: string
  config: string
  price_mode: number
  unit_price: number
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
  mine?: boolean
}
