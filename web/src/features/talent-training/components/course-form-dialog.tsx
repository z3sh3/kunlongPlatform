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

import { adminCreateCourse, adminUpdateCourse } from '../api'
import { COURSE_CATEGORY } from '../constants'
import {
  COURSE_FORM_DEFAULT_VALUES,
  courseFormSchema,
  type CourseFormValues,
} from '../lib/schema'
import type { TrainingCourse } from '../types'

type CourseFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** When set, the dialog edits this course; otherwise it creates one. */
  course?: TrainingCourse | null
}

export function CourseFormDialog(props: CourseFormDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const form = useForm<CourseFormValues>({
    resolver: zodResolver(courseFormSchema),
    defaultValues: COURSE_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    const course = props.course
    form.reset(
      course
        ? {
            title: course.title,
            category: course.category as CourseFormValues['category'],
            description: course.description,
            content: course.content,
            schedule: course.schedule,
            capacity: course.capacity,
            price_mode: course.price_mode,
            price: course.price,
          }
        : COURSE_FORM_DEFAULT_VALUES
    )
  }, [props.course, props.open, form])

  const saveMutation = useMutation({
    mutationFn: (values: CourseFormValues) => {
      if (props.course) {
        return adminUpdateCourse(props.course.id, values)
      }
      return adminCreateCourse(values).then(() => undefined)
    },
    onSuccess: () => {
      toast.success(
        props.course
          ? t('Course updated successfully')
          : t('Course created successfully')
      )
      queryClient.invalidateQueries({ queryKey: ['talent-training', 'courses'] })
      props.onOpenChange(false)
    },
    onError: (error) => handleServerError(error),
  })

  const categoryItems = [
    {
      value: COURSE_CATEGORY.CERTIFICATION,
      labelKey: 'Certification training',
    },
    {
      value: COURSE_CATEGORY.CONSULTATION,
      labelKey: 'Training consultation',
    },
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
            {props.course ? t('Edit course') : t('Create course')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Certification trainings and consultation sessions are published by administrators.'
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
                      placeholder={t('Course title')}
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
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      items={categoryItems.map((item) => ({
                        value: item.value,
                        label: item.labelKey,
                      }))}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder={t('Select category')} />
                      </SelectTrigger>
                      <SelectContent>
                        {categoryItems.map((item) => (
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
                name='capacity'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Capacity')}</FormLabel>
                    <FormControl>
                      <Input type='number' min={0} {...field} />
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
                      placeholder={t('Course summary')}
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
              name='schedule'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Schedule')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('Sessions and timetable')}
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
              name='content'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Content')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('Syllabus and certification details')}
                      rows={4}
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
                name='price'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Price')}</FormLabel>
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
                {props.course ? t('Save changes') : t('Create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
