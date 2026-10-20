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
import { JOB_STATUS } from '../constants'
import type { FinetuneJob } from '../types'

const base = {
  user_id: 2,
  dataset_id: 0,
  dataset_refs: '',
  hyper_params: '',
  result_model: '',
  error_msg: '',
  dataflow_template: '',
  dataflow_params: '',
  service_job_id: '',
  progress: 0,
  output_dir: '',
  usage: '',
  started_at: 0,
  finished_at: 0,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const draft: FinetuneJob = {
  ...base,
  id: 1,
  name: 'my-lora-job',
  base_model: 'gpt-4o-mini',
  method: 'lora',
  status: JOB_STATUS.DRAFT,
}

const submitted: FinetuneJob = {
  ...base,
  id: 2,
  name: 'queued-sft-job',
  base_model: 'gpt-4o',
  method: 'sft',
  status: JOB_STATUS.SUBMITTED,
}

export const JOBS = {
  draft,
  submitted,
  all: [draft, submitted],
}
