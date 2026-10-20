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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { ServerPagination, StaticDataTable } from '@/components/data-table'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  cancelDatasetPrepare,
  datasetPrepareLogsPath,
  deleteDataset,
  getCapabilities,
  installDemoDatasets,
  listDatasets,
  listDemoDatasets,
  startDatasetPrepare,
} from '../api'
import {
  DATASET_SOURCE_OPTIONS,
  JOB_STATUS,
  JOB_STATUS_OPTIONS,
  getOptionLabelKey,
} from '../constants'
import { isTrainingServiceUnavailable, showTrainingError } from '../lib/errors'
import type { DemoDataset, PipelineTemplate, TrainingDataset } from '../types'
import { DatasetFormDialog } from './dataset-form-dialog'
import { LogStreamViewer } from './log-stream-viewer'
import { PipelineParamForm } from './pipeline-param-form'

export function DatasetsPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [formOpen, setFormOpen] = useState(false)
  const [demoOpen, setDemoOpen] = useState(false)
  const [demoSelected, setDemoSelected] = useState<string[]>([])
  const [preparing, setPreparing] = useState<TrainingDataset | null>(null)
  const [logsFor, setLogsFor] = useState<TrainingDataset | null>(null)
  const [deleting, setDeleting] = useState<TrainingDataset | null>(null)
  const [pipelineId, setPipelineId] = useState('')
  const [params, setParams] = useState<Record<string, unknown>>({})
  const [model, setModel] = useState('')

  const datasetsQuery = useQuery({
    queryKey: ['model-training', 'datasets', page, pageSize],
    queryFn: () => listDatasets({ p: page, page_size: pageSize }),
    refetchInterval: (query) => {
      const items = query.state.data?.items ?? []
      const busy = items.some(
        (item) =>
          item.prepare_status === JOB_STATUS.QUEUED ||
          item.prepare_status === JOB_STATUS.RUNNING
      )
      return busy ? 4000 : false
    },
  })

  const capabilitiesQuery = useQuery({
    queryKey: ['model-training', 'capabilities'],
    queryFn: getCapabilities,
    enabled: preparing !== null,
    retry: false,
  })

  const demoQuery = useQuery({
    queryKey: ['model-training', 'demo-datasets'],
    queryFn: listDemoDatasets,
    enabled: demoOpen,
  })

  const demoMutation = useMutation({
    mutationFn: (keys: string[]) => installDemoDatasets(keys),
    onSuccess: (result) => {
      if (result.installed.length > 0) {
        toast.success(
          t('Installed {{count}} demo dataset(s)', {
            count: result.installed.length,
          })
        )
      } else if (result.skipped.length > 0) {
        toast.info(t('Those demo datasets were already installed'))
      }
      if (result.failed.length > 0) {
        toast.error(
          t('{{count}} demo dataset(s) could not be installed', {
            count: result.failed.length,
          })
        )
      }
      setDemoOpen(false)
      setDemoSelected([])
      invalidate()
    },
    onError: (error) => showTrainingError(error),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['model-training', 'datasets'] })
  }

  const prepareMutation = useMutation({
    mutationFn: (payload: {
      dataset: TrainingDataset
      pipeline: string
      params: Record<string, unknown>
      model: string
    }) =>
      startDatasetPrepare(payload.dataset.id, {
        pipeline: payload.pipeline,
        params: payload.params,
        model: payload.model,
      }),
    onSuccess: () => {
      toast.success(t('DataFlow preparation started'))
      setPreparing(null)
      setPipelineId('')
      setParams({})
      setModel('')
      invalidate()
    },
    onError: (error) => showTrainingError(error),
  })

  const cancelMutation = useMutation({
    mutationFn: (dataset: TrainingDataset) => cancelDatasetPrepare(dataset.id),
    onSuccess: () => {
      toast.success(t('Preparation canceled'))
      invalidate()
    },
    onError: (error) => showTrainingError(error),
  })

  const deleteMutation = useMutation({
    mutationFn: (dataset: TrainingDataset) => deleteDataset(dataset.id),
    onSuccess: () => {
      toast.success(t('Dataset deleted'))
      setDeleting(null)
      invalidate()
    },
    onError: (error) => showTrainingError(error),
  })

  const pipelines: PipelineTemplate[] = capabilitiesQuery.data?.pipelines ?? []
  const demoMode = capabilitiesQuery.data?.demo_mode === true
  const activePipeline = pipelines.find((item) => item.id === pipelineId)

  const columns = [
    {
      id: 'name',
      header: t('Name'),
      cell: (dataset: TrainingDataset) => (
        <div className='min-w-0'>
          <p className='truncate font-medium'>{dataset.name}</p>
          <p className='text-muted-foreground truncate text-xs'>
            {t(
              DATASET_SOURCE_OPTIONS.find(
                (option) => option.value === dataset.source
              )?.labelKey ?? 'Upload'
            )}
            {dataset.rows > 0 ? ` · ${dataset.rows} ${t('rows')}` : ''}
          </p>
        </div>
      ),
    },
    {
      id: 'prepare',
      header: t('Preparation'),
      cell: (dataset: TrainingDataset) => {
        if (dataset.prepared) {
          return <Badge variant='outline'>{t('Prepared')}</Badge>
        }
        if (
          dataset.prepare_status === JOB_STATUS.QUEUED ||
          dataset.prepare_status === JOB_STATUS.RUNNING
        ) {
          return (
            <Badge variant='outline'>
              {t(
                getOptionLabelKey(JOB_STATUS_OPTIONS, dataset.prepare_status) ??
                  'Running'
              )}
            </Badge>
          )
        }
        return <span className='text-muted-foreground text-xs'>-</span>
      },
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cell: (dataset: TrainingDataset) => {
        const busy =
          dataset.prepare_status === JOB_STATUS.QUEUED ||
          dataset.prepare_status === JOB_STATUS.RUNNING
        return (
          <div className='flex justify-end gap-1'>
            {dataset.prepare_run_id ? (
              <Button
                variant='ghost'
                size='sm'
                onClick={() => setLogsFor(dataset)}
              >
                {t('Logs')}
              </Button>
            ) : null}
            {!dataset.prepared && !busy ? (
              <Button
                variant='ghost'
                size='sm'
                onClick={() => {
                  setPreparing(dataset)
                  setPipelineId('')
                  setParams({})
                  setModel('')
                }}
              >
                {t('Prepare')}
              </Button>
            ) : null}
            {busy ? (
              <Button
                variant='ghost'
                size='sm'
                disabled={cancelMutation.isPending}
                onClick={() => cancelMutation.mutate(dataset)}
              >
                {t('Cancel')}
              </Button>
            ) : null}
            <Button
              variant='ghost'
              size='sm'
              className='text-destructive hover:text-destructive'
              onClick={() => setDeleting(dataset)}
            >
              {t('Delete')}
            </Button>
          </div>
        )
      },
    },
  ]

  const items = datasetsQuery.data?.items ?? []
  const demoCatalogue = demoQuery.data
  const demoConfigured = demoCatalogue?.configured === true
  const serviceUnavailable = isTrainingServiceUnavailable(datasetsQuery.error)

  return (
    <div className='space-y-4'>
      {serviceUnavailable ? (
        <Alert variant='destructive'>
          <AlertTitle>{t('Training service unavailable')}</AlertTitle>
          <AlertDescription>
            {t(
              'An administrator must configure the model training service before datasets can be managed.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      <div className='flex justify-end gap-2'>
        <Button variant='outline' onClick={() => setDemoOpen(true)}>
          {t('Load demo data')}
        </Button>
        <Button onClick={() => setFormOpen(true)}>
          <Plus />
          {t('Add dataset')}
        </Button>
      </div>

      <StaticDataTable
        columns={columns}
        data={items}
        getRowKey={(dataset) => dataset.id}
        empty={datasetsQuery.isSuccess && items.length === 0}
        emptyContent={t('No datasets yet')}
      />
      {datasetsQuery.data ? (
        <ServerPagination
          page={page}
          pageSize={pageSize}
          total={datasetsQuery.data.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          disabled={datasetsQuery.isFetching}
        />
      ) : null}

      <Dialog open={demoOpen} onOpenChange={setDemoOpen}>
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Demo datasets')}</DialogTitle>
            <DialogDescription>
              {t(
                'Bundled demonstration datasets. Install them to try the training flow without preparing your own data.'
              )}
            </DialogDescription>
          </DialogHeader>

          {demoQuery.isSuccess && !demoConfigured ? (
            <Alert>
              <AlertTitle>{t('Training service not configured')}</AlertTitle>
              <AlertDescription>
                {t(
                  'An administrator must enter the Model Training Service URL and secret in System settings -> Operations before demo data can be installed.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}

          <div className='space-y-2'>
            {(demoCatalogue?.datasets ?? []).map((demo: DemoDataset) => (
              <label
                key={demo.key}
                className='flex cursor-pointer items-start gap-3 rounded-lg border p-3'
              >
                <Checkbox
                  className='mt-1'
                  checked={demo.installed || demoSelected.includes(demo.key)}
                  disabled={demo.installed || demoMutation.isPending}
                  onCheckedChange={(checked) => {
                    setDemoSelected((previous) => {
                      const withoutKey = previous.filter((key) => key !== demo.key)
                      return checked === true ? [...withoutKey, demo.key] : withoutKey
                    })
                  }}
                />
                <span className='min-w-0 flex-1 space-y-1'>
                  <span className='flex flex-wrap items-center gap-2'>
                    <span className='font-medium'>{t(demo.name)}</span>
                    {demo.installed ? (
                      <Badge variant='outline'>{t('Installed')}</Badge>
                    ) : null}
                  </span>
                  <span className='text-muted-foreground block text-xs'>
                    {t(demo.description)}
                  </span>
                  <span className='text-muted-foreground flex flex-wrap gap-2 text-xs'>
                    <span>{demo.format}</span>
                    {demo.rows > 0 ? (
                      <span>
                        {demo.rows} {t('rows')}
                      </span>
                    ) : null}
                    {demo.suggested_pipeline ? (
                      <span>
                        {t('Suggested pipeline: {{pipeline}}', {
                          pipeline: demo.suggested_pipeline,
                        })}
                      </span>
                    ) : null}
                  </span>
                </span>
              </label>
            ))}
          </div>

          {demoQuery.isError ? (
            <p className='text-destructive text-xs'>
              {t('Could not load the demo datasets')}
            </p>
          ) : null}

          <DialogFooter>
            <Button variant='outline' onClick={() => setDemoOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={
                demoSelected.length === 0 ||
                demoMutation.isPending ||
                !demoConfigured
              }
              onClick={() => demoMutation.mutate(demoSelected)}
            >
              {t('Install selected')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <DatasetFormDialog open={formOpen} onOpenChange={setFormOpen} />

      <Dialog
        open={preparing !== null}
        onOpenChange={(open) => {
          if (!open) setPreparing(null)
        }}
      >
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>{t('Prepare dataset with DataFlow')}</DialogTitle>
            <DialogDescription>
              {t(
                'The pipeline runs on the training service and consumes your quota for the model calls it makes.'
              )}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label>{t('Pipeline')}</Label>
              <Select
                value={pipelineId}
                onValueChange={(value) => {
                  const next = value ?? ''
                  setPipelineId(next)
                  const template = pipelines.find((item) => item.id === next)
                  const defaults: Record<string, unknown> = {}
                  for (const param of template?.params ?? []) {
                    if (param.default !== undefined) {
                      defaults[param.key] = param.default
                    }
                  }
                  setParams(defaults)
                }}
                items={pipelines.map((item) => ({
                  value: item.id,
                  label: t(item.name),
                }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('Select a pipeline')} />
                </SelectTrigger>
                <SelectContent>
                  {pipelines.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {t(item.name)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {activePipeline ? (
                <p className='text-muted-foreground text-xs'>
                  {t(activePipeline.description)}
                </p>
              ) : null}
            </div>

            {activePipeline ? (
              <PipelineParamForm
                params={activePipeline.params}
                value={params}
                onChange={setParams}
              />
            ) : null}

            {demoMode ? (
              <Alert>
                <AlertTitle>{t('Demo mode')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'The training service simulates this pipeline, so no cleaning model or GPU is used.'
                  )}
                </AlertDescription>
              </Alert>
            ) : (
              <div className='space-y-2'>
                <Label htmlFor='cleaning-model'>{t('Cleaning model')}</Label>
                <Input
                  id='cleaning-model'
                  value={model}
                  onChange={(event) => setModel(event.target.value)}
                  placeholder='gpt-4o-mini'
                />
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'A platform model name. The pipeline uses a temporary key limited to this model.'
                  )}
                </p>
              </div>
            )}
          </div>

          <DialogFooter>
            <Button variant='outline' onClick={() => setPreparing(null)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={
                prepareMutation.isPending ||
                !pipelineId ||
                (!demoMode && !model.trim())
              }
              onClick={() => {
                if (!preparing) return
                prepareMutation.mutate({
                  dataset: preparing,
                  pipeline: pipelineId,
                  params,
                  model: demoMode ? '' : model.trim(),
                })
              }}
            >
              {t('Start preparation')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={logsFor !== null}
        onOpenChange={(open) => {
          if (!open) setLogsFor(null)
        }}
      >
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('DataFlow log')}</DialogTitle>
            <DialogDescription>
              {logsFor
                ? t('Preparation log for {{name}}', { name: logsFor.name })
                : ''}
            </DialogDescription>
          </DialogHeader>
          {logsFor ? (
            <LogStreamViewer path={datasetPrepareLogsPath(logsFor.id)} />
          ) : null}
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={t('Delete dataset')}
        desc={t(
          'The dataset is removed from the training service. Prepared datasets derived from it are kept.'
        )}
        confirmText={t('Delete')}
        destructive
        handleConfirm={() => {
          if (deleting) deleteMutation.mutate(deleting)
        }}
        isLoading={deleteMutation.isPending}
      />
    </div>
  )
}
