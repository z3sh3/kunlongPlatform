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
  dataset_id: number
  dataset_refs: string
  hyper_params: string
  status: number
  result_model: string
  error_msg: string
  dataflow_template: string
  dataflow_params: string
  service_job_id: string
  progress: number
  output_dir: string
  usage: string
  started_at: number
  finished_at: number
  created_at: string
  updated_at: string
}

export type FinetuneJobFormData = {
  name: string
  base_model: string
  method: string
  dataset_id: number
  hyper_params: string
}

export type TrainingDataset = {
  id: number
  user_id: number
  name: string
  source: string
  source_ref: string
  format: string
  size_bytes: number
  rows: number
  status: number
  error_msg: string
  remote_id: string
  prepared: boolean
  prepared_ref: string
  pipeline: string
  pipeline_params: string
  prepare_run_id: string
  prepare_status: number
  prepare_error: string
  created_at: string
  updated_at: string
}

export type TrainedModel = {
  id: number
  user_id: number
  job_id: number
  name: string
  base_model: string
  method: string
  artifact_kind: string
  artifact_ref: string
  size_bytes: number
  status: number
  channel_id: number
  model_catalog_id: number
  created_at: string
  updated_at: string
}

/** One bundled demonstration dataset offered by the console. */
export type DemoDataset = {
  key: string
  name: string
  description: string
  suggested_pipeline: string
  format: string
  rows: number
  installed: boolean
}

/** The bundled catalogue plus whether it can be installed right now. */
export type DemoCatalogue = {
  configured: boolean
  datasets: DemoDataset[]
}

export type DemoInstallResult = {
  installed: TrainingDataset[]
  skipped: string[]
  failed: Array<{ key: string; message: string }>
}

/** One parameter of a DataFlow pipeline template, as declared by the service. */
export type PipelineParam = {
  key: string
  label: string
  type: 'string' | 'text' | 'int' | 'float' | 'bool' | 'enum'
  required?: boolean
  default?: string | number | boolean | null
  minimum?: number
  maximum?: number
  options?: string[]
  unit?: string
  help?: string
}

export type PipelineTemplate = {
  id: string
  name: string
  description: string
  input_formats?: string[]
  params: PipelineParam[]
}

export type TrainingCapabilities = {
  pipelines: PipelineTemplate[]
  methods: string[]
  stages: string[]
  hyper_params: PipelineParam[]
  /** True when the service simulates preparation and training (no GPU). */
  demo_mode?: boolean
}

export type TrainingServiceHealth = {
  ok: boolean
  /** True when the service simulates preparation and training (no GPU). */
  demo_mode?: boolean
  versions: Record<string, string>
  gpus: Array<Record<string, string>>
  limits: Record<string, number>
  active: Record<string, number>
}

export type TrainingServiceTestResult = {
  configured: boolean
  health: TrainingServiceHealth
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

/** Server errors may carry a stable code the UI can branch on. */
export type ApiErrorResponse = {
  success: false
  message: string
  code?: string
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
  source?: string
  userId?: number
}

export type LogFrame = {
  offset: number
  line: string
}
