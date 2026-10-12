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
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'

import { createDataset, updateDataset } from '../api'
import {
  DATASET_FORM_DEFAULT_VALUES,
  datasetFormSchema,
  type DatasetFormValues,
} from '../lib/schema'
import type { Dataset } from '../types'

type DatasetFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** When set, the dialog edits this dataset; otherwise it creates one. */
  dataset?: Dataset | null
}

export function DatasetFormDialog(props: DatasetFormDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const form = useForm<DatasetFormValues>({
    resolver: zodResolver(datasetFormSchema),
    defaultValues: DATASET_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    const dataset = props.dataset
    form.reset(
      dataset
        ? {
            name: dataset.name,
            description: dataset.description,
            category: dataset.category,
            format: dataset.format,
            file_url: dataset.file_url,
            cover_url: dataset.cover_url,
            size_bytes: dataset.size_bytes,
            price_mode: dataset.price_mode,
            unit_price: dataset.unit_price,
          }
        : DATASET_FORM_DEFAULT_VALUES
    )
  }, [props.dataset, props.open, form])

  const saveMutation = useMutation({
    mutationFn: (values: DatasetFormValues) => {
      if (props.dataset) {
        return updateDataset(props.dataset.id, values)
      }
      return createDataset(values).then(() => undefined)
    },
    onSuccess: () => {
      toast.success(
        props.dataset
          ? t('Dataset updated successfully')
          : t('Dataset created successfully')
      )
      queryClient.invalidateQueries({ queryKey: ['data-market', 'datasets'] })
      queryClient.invalidateQueries({ queryKey: ['data-market', 'grants'] })
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
            {props.dataset ? t('Edit dataset') : t('Create dataset')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Describe the dataset and set its access price. An administrator reviews it before it goes live.'
            )}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
            className='space-y-4'
          >
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Dataset name')}
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='category'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Category')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. vision, nlp')}
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
                name='format'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Format')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. jsonl, csv')}
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
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Description')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('What does this dataset contain?')}
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
              name='file_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('File URL')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Dataset storage address')}
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className='grid grid-cols-1 gap-4 sm:grid-cols-3'>
              <FormField
                control={form.control}
                name='price_mode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Price mode')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        max={1}
                        {...field}
                        value={field.value ?? 0}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='unit_price'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Unit price')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        {...field}
                        value={field.value ?? 0}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='size_bytes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Size (bytes)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        {...field}
                        value={field.value ?? 0}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <DialogFooter>
              <Button type='submit' disabled={saveMutation.isPending}>
                {props.dataset ? t('Save changes') : t('Create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
