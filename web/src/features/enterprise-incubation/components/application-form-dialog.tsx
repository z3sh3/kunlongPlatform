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

import { createApplication, listPrograms } from '../api'
import {
  APPLICATION_FORM_DEFAULT_VALUES,
  applicationFormSchema,
  type ApplicationFormValues,
} from '../lib/schema'

type ApplicationFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Program to preselect when applying from a program row. */
  programId?: number
}

export function ApplicationFormDialog(props: ApplicationFormDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const form = useForm<ApplicationFormValues>({
    resolver: zodResolver(applicationFormSchema),
    defaultValues: APPLICATION_FORM_DEFAULT_VALUES,
  })

  const programsQuery = useQuery({
    queryKey: ['enterprise-incubation', 'programs', 'options'],
    queryFn: () => listPrograms({ p: 1, page_size: 100 }),
    enabled: props.open,
  })

  const programOptions = (programsQuery.data?.items ?? []).map(
    (program) => ({
      value: String(program.id),
      label: program.title,
    })
  )

  useEffect(() => {
    if (props.open) {
      form.reset({
        ...APPLICATION_FORM_DEFAULT_VALUES,
        program_id: props.programId ?? 0,
      })
    }
  }, [props.open, props.programId, form])

  const saveMutation = useMutation({
    mutationFn: (values: ApplicationFormValues) =>
      createApplication(values).then(() => undefined),
    onSuccess: () => {
      toast.success(t('Application submitted'))
      queryClient.invalidateQueries({
        queryKey: ['enterprise-incubation', 'applications'],
      })
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
          <DialogTitle>{t('Apply for a program')}</DialogTitle>
          <DialogDescription>
            {t(
              'Submit your company information. An administrator will review the application.'
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
              name='program_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Program')}</FormLabel>
                  <Select
                    value={field.value > 0 ? String(field.value) : undefined}
                    onValueChange={(value) => field.onChange(Number(value))}
                    items={programOptions}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t('Select program')} />
                    </SelectTrigger>
                    <SelectContent>
                      {programOptions.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='company_name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Company name')}</FormLabel>
                    <FormControl>
                      <Input {...field} value={field.value ?? ''} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='contact_name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Contact name')}</FormLabel>
                    <FormControl>
                      <Input {...field} value={field.value ?? ''} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='contact_phone'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Contact phone')}</FormLabel>
                    <FormControl>
                      <Input {...field} value={field.value ?? ''} />
                    </FormControl>
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
                      placeholder={t('Company profile and needs')}
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
                {t('Submit')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
