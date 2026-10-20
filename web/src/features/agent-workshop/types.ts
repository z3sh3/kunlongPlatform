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
export type DifyAgent = {
  id: string
  name: string
  description: string
  /** Dify app mode, e.g. chat, advanced-chat, agent, workflow. */
  mode: string
  icon_type: string
  icon: string
  icon_background: string
  created_at: string
  updated_at: string
  /** Console deep link for the app, relative to console_base_url. */
  console_path: string
}

export type ApiResponse<T = unknown> = {
  success: boolean
  message: string
  data: T
}

export type DifyAgentsPage = {
  page: number
  page_size: number
  total: number
  items: DifyAgent[]
  /** Public origin of the Dify console; empty when the admin left it blank. */
  console_base_url: string
}

export type DifyAgentListParams = {
  p?: number
  page_size?: number
  keyword?: string
  mode?: string
  mine?: boolean
}

export type DifyAgentAccess = {
  name: string
  mode: string
  /** Published web-app URL; empty when site access is off or unpublished. */
  webapp_url: string
  /** Service API base URL; empty when API access is off. */
  api_base_url: string
  /** Console deep link relative to the console origin. */
  console_path: string
}
