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
import { useMutation, useQueryClient } from '@tanstack/react-query'
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
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'

import { createJob, updateJob } from '../api'
import { FINETUNE_METHODS } from '../constants'
import { JOB_FORM_DEFAULT_VALUES, jobFormSchema, type JobFormValues } from '../lib/schema'
import type { FinetuneJob } from '../types'

type JobFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** When set, the dialog edits this job; otherwise it creates one. */
  job?: FinetuneJob | null
}

export function JobFormDialog(props: JobFormDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const form = useForm<JobFormValues>({
    resolver: zodResolver(jobFormSchema),
    defaultValues: JOB_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    const job = props.job
    form.reset(
      job
        ? {
            name: job.name,
            base_model: job.base_model,
            method: (FINETUNE_METHODS as readonly string[]).includes(
              job.method
            )
              ? (job.method as JobFormValues['method'])
              : 'lora',
            dataset_refs: job.dataset_refs,
            hyper_params: job.hyper_params,
          }
        : JOB_FORM_DEFAULT_VALUES
    )
  }, [props.job, props.open, form])

  const saveMutation = useMutation({
    mutationFn: (values: JobFormValues) => {
      if (props.job) {
        return updateJob(props.job.id, values)
      }
      return createJob(values).then(() => undefined)
    },
    onSuccess: () => {
      toast.success(
        props.job
          ? t('Fine-tuning job updated successfully')
          : t('Fine-tuning job created successfully')
      )
      queryClient.invalidateQueries({ queryKey: ['model-training', 'jobs'] })
      props.onOpenChange(false)
    },
    onError: (error) => handleServerError(error),
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (!saveMutation.isPending) props.onOpenChange(open)
      }}
    >
      <DialogContent className='sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>
            {props.job ? t('Edit fine-tuning job') : t('Create fine-tuning job')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Pick a base model, a method and dataset references. Training dispatch is handled by an administrator.'
            )}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
            className='space-y-4'
          >
            <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Job name')}
                        {...field}
                        value={field.value ?? ''}
                      />
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
                      <Input
                        placeholder={t('e.g. gpt-4o-mini')}
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='method'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Fine-tuning method')}</FormLabel>
                  <Select
                    value={field.value}
                    onValueChange={field.onChange}
                    items={FINETUNE_METHODS.map((method) => ({
                      value: method,
                      label: method,
                    }))}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t('Select method')} />
                    </SelectTrigger>
                    <SelectContent>
                      {FINETUNE_METHODS.map((method) => (
                        <SelectItem key={method} value={method}>
                          {method}
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
              name='dataset_refs'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Dataset references')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('Dataset references (JSON array)')}
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='hyper_params'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Hyperparameters')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('Hyperparameters (JSON object)')}
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter>
              <Button type='submit' disabled={saveMutation.isPending}>
                {props.job ? t('Save changes') : t('Create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
