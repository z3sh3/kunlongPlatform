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
import { api, getFreshAuthHeaders } from '@/lib/api'

import { TrainingServiceError } from './lib/errors'
import type {
  AdminJobStatusPayload,
  ApiErrorResponse,
  ApiResponse,
  DemoCatalogue,
  DemoInstallResult,
  FinetuneJob,
  FinetuneJobFormData,
  ListParams,
  LogFrame,
  PagedData,
  TrainingCapabilities,
  TrainingDataset,
  TrainingServiceTestResult,
  TrainedModel,
} from './types'

const BASE_URL = '/api/model-training'

/**
 * Unwraps the `{success, message, data, code}` envelope. The console shows
 * failures with `success: false` on an HTTP 200 response, so every call has to
 * check the flag rather than the status code.
 */
function unwrap<T>(payload: ApiResponse<T> | ApiErrorResponse): T {
  if (!payload?.success) {
    throw new TrainingServiceError(
      payload?.message || 'Request failed',
      (payload as ApiErrorResponse)?.code
    )
  }
  return (payload as ApiResponse<T>).data
}

function buildQuery(params: ListParams, withUserId = false): string {
  const query = new URLSearchParams()
  query.set('p', String(params.p ?? 1))
  query.set('page_size', String(params.page_size ?? 10))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.status !== undefined) {
    query.set('status', String(params.status))
  }
  if (params.source) query.set('source', params.source)
  if (withUserId && params.userId !== undefined) {
    query.set('user_id', String(params.userId))
  }
  return query.toString()
}

// ---------------------------------------------------------------------------
// Service capabilities
// ---------------------------------------------------------------------------

export async function getCapabilities(): Promise<TrainingCapabilities> {
  const res = await api.get(`${BASE_URL}/capabilities`)
  return unwrap<TrainingCapabilities>(res.data)
}

export async function testTrainingService(): Promise<TrainingServiceTestResult> {
  const res = await api.post(`${BASE_URL}/admin/training-service/test`)
  return unwrap<TrainingServiceTestResult>(res.data)
}

// ---------------------------------------------------------------------------
// Datasets
// ---------------------------------------------------------------------------

export async function listDatasets(
  params: ListParams
): Promise<PagedData<TrainingDataset>> {
  const res = await api.get(`${BASE_URL}/datasets?${buildQuery(params)}`)
  return unwrap<PagedData<TrainingDataset>>(res.data)
}

export async function uploadDataset(
  file: File,
  name: string
): Promise<TrainingDataset> {
  const form = new FormData()
  form.append('file', file)
  form.append('name', name)
  const res = await api.post(`${BASE_URL}/datasets`, form)
  return unwrap<TrainingDataset>(res.data)
}

export async function createDatasetFromURL(payload: {
  url: string
  name?: string
  market_dataset_id?: number
}): Promise<TrainingDataset> {
  const res = await api.post(`${BASE_URL}/datasets/from-url`, payload)
  return unwrap<TrainingDataset>(res.data)
}

export async function deleteDataset(id: number): Promise<void> {
  const res = await api.delete(`${BASE_URL}/datasets/${id}`)
  unwrap<null>(res.data)
}

export async function startDatasetPrepare(
  id: number,
  payload: { pipeline: string; params: Record<string, unknown>; model: string }
): Promise<{ prepare_run_id: string }> {
  const res = await api.post(`${BASE_URL}/datasets/${id}/prepare`, payload)
  return unwrap<{ prepare_run_id: string }>(res.data)
}

export async function getDatasetPrepare(
  id: number
): Promise<TrainingDataset> {
  const res = await api.get(`${BASE_URL}/datasets/${id}/prepare`)
  return unwrap<TrainingDataset>(res.data)
}

export async function cancelDatasetPrepare(id: number): Promise<void> {
  const res = await api.post(`${BASE_URL}/datasets/${id}/prepare/cancel`)
  unwrap<null>(res.data)
}

// ---------------------------------------------------------------------------
// Bundled demo datasets
// ---------------------------------------------------------------------------

export async function listDemoDatasets(): Promise<DemoCatalogue> {
  const res = await api.get(`${BASE_URL}/demo-datasets`)
  return unwrap<DemoCatalogue>(res.data)
}

/** Installs the selected demo datasets (all of them when `keys` is empty). */
export async function installDemoDatasets(
  keys: string[]
): Promise<DemoInstallResult> {
  const res = await api.post(`${BASE_URL}/demo-datasets/install`, { keys })
  return unwrap<DemoInstallResult>(res.data)
}

// ---------------------------------------------------------------------------
// Fine-tuning jobs
// ---------------------------------------------------------------------------

export async function listJobs(
  params: ListParams
): Promise<PagedData<FinetuneJob>> {
  const res = await api.get(`${BASE_URL}/jobs?${buildQuery(params)}`)
  return unwrap<PagedData<FinetuneJob>>(res.data)
}

export async function getJob(id: number): Promise<FinetuneJob> {
  const res = await api.get(`${BASE_URL}/jobs/${id}`)
  return unwrap<FinetuneJob>(res.data)
}

export async function createJob(
  data: FinetuneJobFormData
): Promise<FinetuneJob> {
  const res = await api.post(`${BASE_URL}/jobs`, data)
  return unwrap<FinetuneJob>(res.data)
}

export async function updateJob(
  id: number,
  data: FinetuneJobFormData
): Promise<void> {
  const res = await api.put(`${BASE_URL}/jobs/${id}`, data)
  unwrap<FinetuneJob>(res.data)
}

export async function deleteJob(id: number): Promise<void> {
  const res = await api.delete(`${BASE_URL}/jobs/${id}`)
  unwrap<null>(res.data)
}

export async function submitJob(id: number): Promise<void> {
  const res = await api.post(`${BASE_URL}/jobs/${id}/submit`)
  unwrap<FinetuneJob>(res.data)
}

export async function cancelJob(id: number): Promise<void> {
  const res = await api.post(`${BASE_URL}/jobs/${id}/cancel`)
  unwrap<null>(res.data)
}

export async function adminListJobs(
  params: ListParams
): Promise<PagedData<FinetuneJob>> {
  const res = await api.get(
    `${BASE_URL}/admin/jobs?${buildQuery(params, true)}`
  )
  return unwrap<PagedData<FinetuneJob>>(res.data)
}

export async function adminUpdateJobStatus(
  id: number,
  payload: AdminJobStatusPayload
): Promise<void> {
  const res = await api.put(`${BASE_URL}/admin/jobs/${id}/status`, payload)
  unwrap<null>(res.data)
}

// ---------------------------------------------------------------------------
// Trained model assets
// ---------------------------------------------------------------------------

export async function listArtifacts(
  params: ListParams
): Promise<PagedData<TrainedModel>> {
  const res = await api.get(`${BASE_URL}/artifacts?${buildQuery(params)}`)
  return unwrap<PagedData<TrainedModel>>(res.data)
}

export async function registerArtifact(
  jobId: number,
  name: string
): Promise<TrainedModel> {
  const res = await api.post(`${BASE_URL}/jobs/${jobId}/artifact`, { name })
  return unwrap<TrainedModel>(res.data)
}

export async function promoteArtifactToModel(
  artifactId: number,
  payload: { description?: string; tags?: string }
): Promise<void> {
  const res = await api.post(
    `${BASE_URL}/admin/artifacts/${artifactId}/model`,
    payload
  )
  unwrap<unknown>(res.data)
}

export async function registerArtifactChannel(
  artifactId: number,
  payload: { base_url: string; key: string; group?: string; name?: string }
): Promise<{ channel_id: number }> {
  const res = await api.post(
    `${BASE_URL}/admin/artifacts/${artifactId}/channel`,
    payload
  )
  return unwrap<{ channel_id: number }>(res.data)
}

// ---------------------------------------------------------------------------
// Log streaming
// ---------------------------------------------------------------------------

export type LogStreamHandlers = {
  onFrame: (frame: LogFrame) => void
  onDone: () => void
  onError: (message: string) => void
}

/**
 * Reads a server-sent event log stream.
 *
 * `EventSource` cannot be used because console authentication travels in a
 * header, so the stream is read with fetch() and parsed frame by frame. The
 * caller aborts through `signal` and resumes by requesting the last offset.
 */
export async function streamLogs(
  path: string,
  offset: number,
  handlers: LogStreamHandlers,
  signal: AbortSignal
): Promise<void> {
  const headers = await getFreshAuthHeaders()
  let response: Response
  try {
    response = await fetch(`${path}?offset=${offset}`, {
      method: 'GET',
      credentials: 'include',
      headers: { ...headers, Accept: 'text/event-stream' },
      signal,
    })
  } catch (error) {
    if (signal.aborted) return
    handlers.onError(
      error instanceof Error ? error.message : 'Failed to open the log stream'
    )
    return
  }

  if (!response.ok || !response.body) {
    handlers.onError(`HTTP ${response.status}: ${response.statusText}`)
    return
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) return
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''
    for (const line of lines) {
      if (!line.startsWith('data: ')) continue
      const data = line.slice(6).trim()
      if (!data) continue
      if (data === '[DONE]') {
        handlers.onDone()
        return
      }
      try {
        const frame = JSON.parse(data) as LogFrame | { error?: string }
        if ('error' in frame && frame.error) {
          handlers.onError(frame.error)
          return
        }
        handlers.onFrame(frame as LogFrame)
      } catch {
        // A malformed frame is skipped: the stream itself stays usable.
      }
    }
  }
}

export function jobLogsPath(jobId: number): string {
  return `${BASE_URL}/jobs/${jobId}/logs`
}

export function datasetPrepareLogsPath(datasetId: number): string {
  return `${BASE_URL}/datasets/${datasetId}/prepare/logs`
}
