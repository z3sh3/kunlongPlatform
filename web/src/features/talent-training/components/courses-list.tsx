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
import dayjs from 'dayjs'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ServerPagination, StaticDataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { handleServerError } from '@/lib/handle-server-error'

import {
  adminDeleteCourse,
  adminListCourses,
  adminUpdateCourseStatus,
  enroll,
  listCourses,
} from '../api'
import {
  COURSE_CATEGORY,
  PRICE_MODE,
  PUBLISH_STATUS,
  PUBLISH_STATUS_OPTIONS,
  getOptionLabelKey,
} from '../constants'
import type { TrainingCourse } from '../types'
import { CourseFormDialog } from './course-form-dialog'

type CoursesListMode = 'browse' | 'admin'

type CoursesListProps = {
  mode: CoursesListMode
}

export function CoursesList(props: CoursesListProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<TrainingCourse | null>(null)

  const isAdmin = props.mode === 'admin'

  const coursesQuery = useQuery({
    queryKey: ['talent-training', 'courses', props.mode, page, pageSize],
    queryFn: () =>
      isAdmin
        ? adminListCourses({ p: page, page_size: pageSize })
        : listCourses({ p: page, page_size: pageSize }),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['talent-training', 'courses'] })
  }

  const enrollMutation = useMutation({
    mutationFn: (id: number) => enroll(id),
    onSuccess: () => {
      toast.success(t('Enrollment request submitted'))
      invalidate()
      queryClient.invalidateQueries({
        queryKey: ['talent-training', 'enrollments'],
      })
    },
    onError: (error) => handleServerError(error),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => adminDeleteCourse(id),
    onSuccess: () => {
      toast.success(t('Course deleted'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const statusMutation = useMutation({
    mutationFn: (values: { id: number; status: number }) =>
      adminUpdateCourseStatus(values.id, values.status),
    onSuccess: () => {
      toast.success(t('Course status updated'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const renderCategoryBadge = (course: TrainingCourse) => (
    <Badge variant='outline'>
      {course.category === COURSE_CATEGORY.CERTIFICATION
        ? t('Certification training')
        : t('Training consultation')}
    </Badge>
  )

  const renderAdminStatusButton = (course: TrainingCourse) => {
    const isPublished = course.status === PUBLISH_STATUS.PUBLISHED
    return (
      <Button
        variant='ghost'
        size='sm'
        onClick={() =>
          statusMutation.mutate({
            id: course.id,
            status: isPublished
              ? PUBLISH_STATUS.OFFLINE
              : PUBLISH_STATUS.PUBLISHED,
          })
        }
        disabled={statusMutation.isPending}
      >
        {isPublished ? t('Take offline') : t('Publish')}
      </Button>
    )
  }

  const columns = [
    {
      id: 'title',
      header: t('Title'),
      cell: (course: TrainingCourse) => (
        <div className='min-w-0'>
          <p className='truncate font-medium'>{course.title}</p>
          {course.description ? (
            <p className='text-muted-foreground truncate text-xs'>
              {course.description}
            </p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'category',
      header: t('Category'),
      cell: renderCategoryBadge,
    },
    ...(isAdmin
      ? [
          {
            id: 'status',
            header: t('Status'),
            cell: (course: TrainingCourse) => (
              <Badge variant='outline'>
                {t(
                  getOptionLabelKey(PUBLISH_STATUS_OPTIONS, course.status) ??
                    'Draft'
                )}
              </Badge>
            ),
          },
        ]
      : []),
    {
      id: 'capacity',
      header: t('Capacity'),
      cell: (course: TrainingCourse) =>
        course.capacity > 0 ? course.capacity.toLocaleString() : t('Unlimited'),
    },
    {
      id: 'price',
      header: t('Price'),
      cell: (course: TrainingCourse) =>
        course.price_mode === PRICE_MODE.PAID
          ? course.price.toLocaleString()
          : t('Free'),
    },
    {
      id: 'created_at',
      header: t('Created'),
      cell: (course: TrainingCourse) =>
        course.created_at
          ? dayjs(course.created_at).format('YYYY-MM-DD')
          : '-',
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cell: (course: TrainingCourse) => (
        <div className='flex justify-end gap-1'>
          {props.mode === 'browse' ? (
            <Button
              size='sm'
              onClick={() => enrollMutation.mutate(course.id)}
              disabled={enrollMutation.isPending}
            >
              {t('Enroll')}
            </Button>
          ) : null}
          {isAdmin ? (
            <>
              <Button
                variant='ghost'
                size='sm'
                onClick={() => {
                  setEditing(course)
                  setFormOpen(true)
                }}
              >
                {t('Edit')}
              </Button>
              {renderAdminStatusButton(course)}
              <Button
                variant='ghost'
                size='sm'
                className='text-destructive hover:text-destructive'
                onClick={() => deleteMutation.mutate(course.id)}
                disabled={deleteMutation.isPending}
              >
                {t('Delete')}
              </Button>
            </>
          ) : null}
        </div>
      ),
    },
  ]

  const items = coursesQuery.data?.items ?? []

  return (
    <div className='space-y-4'>
      {isAdmin ? (
        <div className='flex justify-end'>
          <Button
            onClick={() => {
              setEditing(null)
              setFormOpen(true)
            }}
          >
            <Plus />
            {t('Create course')}
          </Button>
        </div>
      ) : null}
      <StaticDataTable
        columns={columns}
        data={items}
        getRowKey={(course) => course.id}
        empty={coursesQuery.isSuccess && items.length === 0}
        emptyContent={t('No courses yet')}
      />
      {coursesQuery.data ? (
        <ServerPagination
          page={page}
          pageSize={pageSize}
          total={coursesQuery.data.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          disabled={coursesQuery.isFetching}
        />
      ) : null}
      <CourseFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        course={editing}
      />
    </div>
  )
}
