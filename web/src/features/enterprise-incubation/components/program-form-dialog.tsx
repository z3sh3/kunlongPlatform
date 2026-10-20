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
import { Textarea } from '@/components/ui/textarea'
import { MediaUpload } from '@/components/media-upload'
import { handleServerError } from '@/lib/handle-server-error'

import { adminCreateProgram, adminUpdateProgram } from '../api'
import { PROGRAM_TYPE } from '../constants'
import {
  PROGRAM_FORM_DEFAULT_VALUES,
  programFormSchema,
  type ProgramFormValues,
} from '../lib/schema'
import type { IncubationProgram } from '../types'

type ProgramFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** When set, the dialog edits this program; otherwise it creates one. */
  program?: IncubationProgram | null
}

export function ProgramFormDialog(props: ProgramFormDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const form = useForm<ProgramFormValues>({
    resolver: zodResolver(programFormSchema),
    defaultValues: PROGRAM_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    const program = props.program
    form.reset(
      program
        ? {
            title: program.title,
            type: (Object.values(PROGRAM_TYPE) as string[]).includes(
              program.type
            )
              ? (program.type as ProgramFormValues['type'])
              : PROGRAM_TYPE.INCUBATION,
            summary: program.summary ?? '',
            cover_url: program.cover_url ?? '',
            description: program.description,
            requirements: program.requirements,
          }
        : PROGRAM_FORM_DEFAULT_VALUES
    )
  }, [props.program, props.open, form])

  const saveMutation = useMutation({
    mutationFn: (values: ProgramFormValues) => {
      if (props.program) {
        return adminUpdateProgram(props.program.id, values)
      }
      return adminCreateProgram(values).then(() => undefined)
    },
    onSuccess: () => {
      toast.success(
        props.program
          ? t('Program updated successfully')
          : t('Program created successfully')
      )
      queryClient.invalidateQueries({
        queryKey: ['enterprise-incubation', 'programs'],
      })
      props.onOpenChange(false)
    },
    onError: (error) => handleServerError(error),
  })

  const typeItems = [
    { value: PROGRAM_TYPE.INCUBATION, labelKey: 'Enterprise incubation' },
    { value: PROGRAM_TYPE.SME_SUPPORT, labelKey: 'SME support' },
    { value: PROGRAM_TYPE.GOV_PARK, labelKey: 'Government & park programs' },
  ]

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
            {props.program ? t('Edit program') : t('Create program')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Incubation, SME support and government or park special programs are published by administrators.'
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
              name='title'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Title')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Program title')}
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
              name='type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Program type')}</FormLabel>
                  <Select
                    value={field.value}
                    onValueChange={field.onChange}
                    items={typeItems.map((item) => ({
                      value: item.value,
                      label: item.labelKey,
                    }))}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t('Select type')} />
                    </SelectTrigger>
                    <SelectContent>
                      {typeItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {t(item.labelKey)}
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
              name='cover_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Cover image URL')}</FormLabel>
                  <FormControl>
                    <MediaUpload
                      value={field.value ?? ''}
                      onChange={field.onChange}
                      module='enterprise'
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Optional. An http(s) link or a site path starting with /. A gradient placeholder is shown when empty.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='summary'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Card summary')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('e.g. A 6-month incubation track with mentors and funding')}
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Short introduction shown on the card, up to 500 characters.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Description')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('What does this program offer?')}
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
              name='requirements'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Requirements')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('Eligibility and application requirements')}
                      rows={4}
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
                {props.program ? t('Save changes') : t('Create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
