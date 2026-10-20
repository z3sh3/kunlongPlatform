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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

import { createJob, listDatasets, updateJob } from '../api'
import { FINETUNE_METHOD_OPTIONS, FINETUNE_METHODS } from '../constants'
import { showTrainingError } from '../lib/errors'
import {
  JOB_FORM_DEFAULT_VALUES,
  LR_SCHEDULERS,
  jobFormSchema,
  parseHyperParamsJSON,
  toHyperParamsJSON,
  type JobFormValues,
} from '../lib/schema'
import type { FinetuneJob } from '../types'

type JobFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** When set, the dialog edits this job; otherwise it creates one. */
  job?: FinetuneJob | null
}

/** Numeric inputs report strings, so the form value is converted explicitly. */
function toNumber(value: string): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

export function JobFormDialog(props: JobFormDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const form = useForm<JobFormValues>({
    resolver: zodResolver(jobFormSchema),
    defaultValues: JOB_FORM_DEFAULT_VALUES,
  })

  const datasetsQuery = useQuery({
    queryKey: ['model-training', 'datasets', 'for-job'],
    queryFn: () => listDatasets({ p: 1, page_size: 100 }),
    enabled: props.open,
  })

  useEffect(() => {
    if (!props.open) return
    const job = props.job
    form.reset(
      job
        ? {
            ...JOB_FORM_DEFAULT_VALUES,
            name: job.name,
            base_model: job.base_model,
            method: (FINETUNE_METHODS as readonly string[]).includes(job.method)
              ? (job.method as JobFormValues['method'])
              : 'lora',
            dataset_id: job.dataset_id,
            ...parseHyperParamsJSON(job.hyper_params),
          }
        : JOB_FORM_DEFAULT_VALUES
    )
  }, [props.open, props.job, form])

  const mutation = useMutation({
    mutationFn: async (values: JobFormValues) => {
      const payload = {
        name: values.name,
        base_model: values.base_model,
        method: values.method,
        dataset_id: values.dataset_id,
        hyper_params: toHyperParamsJSON(values),
      }
      if (props.job) {
        await updateJob(props.job.id, payload)
        return
      }
      await createJob(payload)
    },
    onSuccess: () => {
      toast.success(
        props.job ? t('Fine-tuning job updated') : t('Fine-tuning job created')
      )
      queryClient.invalidateQueries({ queryKey: ['model-training', 'jobs'] })
      queryClient.invalidateQueries({
        queryKey: ['model-training', 'datasets', 'for-job'],
      })
      props.onOpenChange(false)
    },
    onError: (error) => showTrainingError(error),
  })

  const readyDatasets = (datasetsQuery.data?.items ?? []).filter(
    (dataset) => dataset.status === 1
  )

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {props.job ? t('Edit fine-tuning job') : t('Create fine-tuning job')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Pick a prepared dataset and set the training hyper-parameters. The job runs on the training service.'
            )}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
            className='space-y-6'
          >
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('my-lora-job')} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='base_model'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Base model')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='Qwen/Qwen3-4B-Instruct-2507' />
                  </FormControl>
                  <FormDescription>
                    {t('Hugging Face or ModelScope identifier loaded by the training host.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className='grid gap-4 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='method'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Method')}</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      items={FINETUNE_METHOD_OPTIONS.map((option) => ({
                        value: option.value,
                        label: t(option.labelKey),
                      }))}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {FINETUNE_METHOD_OPTIONS.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {t(option.labelKey)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='dataset_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Dataset')}</FormLabel>
                    <Select
                      value={field.value > 0 ? String(field.value) : ''}
                      onValueChange={(value) => field.onChange(Number(value))}
                      items={readyDatasets.map((dataset) => ({
                        value: String(dataset.id),
                        label: dataset.name,
                      }))}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t('Select a dataset')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {readyDatasets.map((dataset) => (
                          <SelectItem key={dataset.id} value={String(dataset.id)}>
                            {dataset.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {readyDatasets.length === 0
                        ? t('Upload or prepare a dataset first.')
                        : t('A DataFlow-prepared dataset is preferred.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className='grid gap-4 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='lora_rank'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('LoRA rank')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={256}
                        value={field.value}
                        onChange={(event) => field.onChange(toNumber(event.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='cutoff_len'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Cutoff length')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={128}
                        max={32768}
                        value={field.value}
                        onChange={(event) => field.onChange(toNumber(event.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='num_train_epochs'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Epochs')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.1'
                        min={0.1}
                        max={20}
                        value={field.value}
                        onChange={(event) => field.onChange(toNumber(event.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='learning_rate'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Learning rate')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.00001'
                        value={field.value}
                        onChange={(event) => field.onChange(toNumber(event.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='per_device_train_batch_size'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Batch size per device')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={128}
                        value={field.value}
                        onChange={(event) => field.onChange(toNumber(event.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='gradient_accumulation_steps'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Gradient accumulation steps')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={256}
                        value={field.value}
                        onChange={(event) => field.onChange(toNumber(event.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='lr_scheduler_type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Learning rate scheduler')}</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      items={LR_SCHEDULERS.map((scheduler) => ({
                        value: scheduler,
                        label: scheduler,
                      }))}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {LR_SCHEDULERS.map((scheduler) => (
                          <SelectItem key={scheduler} value={scheduler}>
                            {scheduler}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='bf16'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-3'>
                    <div className='space-y-0.5'>
                      <FormLabel>{t('BF16')}</FormLabel>
                      <FormDescription>
                        {t('Requires an Ampere or newer GPU.')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </div>

            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => props.onOpenChange(false)}
              >
                {t('Cancel')}
              </Button>
              <Button type='submit' disabled={mutation.isPending}>
                {props.job ? t('Save') : t('Create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
