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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { jobLogsPath, registerArtifact } from '../api'
import {
  JOB_STATUS,
  JOB_STATUS_OPTIONS,
  getOptionLabelKey,
  isActiveJobStatus,
} from '../constants'
import { showTrainingError } from '../lib/errors'
import type { FinetuneJob } from '../types'
import { LogStreamViewer } from './log-stream-viewer'

type JobDetailDrawerProps = {
  job: FinetuneJob | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function JobDetailDrawer(props: JobDetailDrawerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [artifactName, setArtifactName] = useState('')

  const job = props.job

  const artifactMutation = useMutation({
    mutationFn: (name: string) => registerArtifact(job?.id ?? 0, name),
    onSuccess: () => {
      toast.success(t('Model asset registered'))
      queryClient.invalidateQueries({ queryKey: ['model-training', 'artifacts'] })
      setArtifactName('')
    },
    onError: (error) => showTrainingError(error),
  })

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className='w-full overflow-y-auto sm:max-w-xl'>
        <SheetHeader>
          <SheetTitle>{job?.name ?? t('Fine-tuning job')}</SheetTitle>
          <SheetDescription>
            {job
              ? `${job.base_model} · ${job.method}`
              : t('Select a job to inspect it')}
          </SheetDescription>
        </SheetHeader>

        {job ? (
          <div className='space-y-4 px-4 pb-4'>
            <Tabs defaultValue='overview'>
              <TabsList>
                <TabsTrigger value='overview'>{t('Overview')}</TabsTrigger>
                <TabsTrigger value='logs'>{t('Logs')}</TabsTrigger>
              </TabsList>
              <TabsContent value='overview' className='mt-4 space-y-3 text-sm'>
                <div className='flex items-center justify-between'>
                  <span className='text-muted-foreground'>{t('Status')}</span>
                  <span>
                    {t(getOptionLabelKey(JOB_STATUS_OPTIONS, job.status) ?? 'Draft')}
                  </span>
                </div>
                {isActiveJobStatus(job.status) ? (
                  <div className='space-y-1'>
                    <div className='flex items-center justify-between'>
                      <span className='text-muted-foreground'>{t('Progress')}</span>
                      <span>{job.progress}%</span>
                    </div>
                    <Progress value={job.progress} />
                  </div>
                ) : null}
                <div className='flex items-center justify-between'>
                  <span className='text-muted-foreground'>{t('Dataset')}</span>
                  <span className='truncate'>{job.dataset_id || '-'}</span>
                </div>
                <div className='flex items-center justify-between'>
                  <span className='text-muted-foreground'>
                    {t('DataFlow pipeline')}
                  </span>
                  <span>{job.dataflow_template || '-'}</span>
                </div>
                <div className='flex items-center justify-between'>
                  <span className='text-muted-foreground'>{t('Output directory')}</span>
                  <span className='truncate'>{job.output_dir || '-'}</span>
                </div>
                {job.error_msg ? (
                  <div className='space-y-1'>
                    <span className='text-muted-foreground'>{t('Failure reason')}</span>
                    <p className='text-destructive text-xs break-all'>
                      {job.error_msg}
                    </p>
                  </div>
                ) : null}
                {job.usage ? (
                  <div className='space-y-1'>
                    <span className='text-muted-foreground'>{t('Recorded usage')}</span>
                    <pre className='bg-muted/40 rounded p-2 text-xs break-all whitespace-pre-wrap'>
                      {job.usage}
                    </pre>
                  </div>
                ) : null}

                {job.status === JOB_STATUS.SUCCEEDED ? (
                  <div className='space-y-2 rounded-lg border p-3'>
                    <p className='text-sm font-medium'>{t('Register model asset')}</p>
                    <Input
                      value={artifactName}
                      onChange={(event) => setArtifactName(event.target.value)}
                      placeholder={job.result_model || job.name}
                    />
                    <Button
                      size='sm'
                      disabled={artifactMutation.isPending}
                      onClick={() =>
                        artifactMutation.mutate(
                          artifactName.trim() || job.result_model || job.name
                        )
                      }
                    >
                      {t('Register')}
                    </Button>
                  </div>
                ) : null}
              </TabsContent>
              <TabsContent value='logs' className='mt-4'>
                <LogStreamViewer path={jobLogsPath(job.id)} />
              </TabsContent>
            </Tabs>
          </div>
        ) : null}

        <SheetFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Close')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
