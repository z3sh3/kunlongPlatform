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

import { ContentCoverCard } from '@/components/content-cover-card'
import { ServerPagination } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { handleServerError } from '@/lib/handle-server-error'

import {
  adminDeleteCourse,
  adminListCourses,
  adminSeedDemoCourses,
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

  // Publishes the three bundled demo cases (course + one enrollment per review
  // stage). Idempotent: existing records are reported, not duplicated.
  const seedDemoMutation = useMutation({
    mutationFn: () => adminSeedDemoCourses(),
    onSuccess: (result) => {
      if (result.created.length > 0) {
        toast.success(
          t('Created {{count}} demo case(s)', {
            count: result.created.length,
          })
        )
      } else {
        toast.info(t('Demo cases already exist'))
      }
      invalidate()
      queryClient.invalidateQueries({
        queryKey: ['talent-training', 'enrollments'],
      })
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

  const renderActions = (course: TrainingCourse) => {
    if (!isAdmin) {
      return (
        <Button
          size='sm'
          onClick={() => enrollMutation.mutate(course.id)}
          disabled={enrollMutation.isPending}
        >
          {t('Enroll')}
        </Button>
      )
    }
    return (
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
    )
  }

  const items = coursesQuery.data?.items ?? []
  const isEmpty = coursesQuery.isSuccess && items.length === 0

  return (
    <div className='space-y-4'>
      {isAdmin ? (
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Demo cases cover certification and consultation courses with enrollments at each review stage.'
            )}
          </p>
          <div className='flex flex-wrap justify-end gap-2'>
            <Button
              variant='outline'
              disabled={seedDemoMutation.isPending}
              onClick={() => seedDemoMutation.mutate()}
            >
              {t('Generate demo cases')}
            </Button>
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
        </div>
      ) : null}

      {isEmpty ? (
        <p className='text-muted-foreground py-16 text-center text-sm'>
          {t('No courses yet')}
        </p>
      ) : (
        <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
          {items.map((course) => (
            <ContentCoverCard
              key={course.id}
              coverUrl={course.cover_url}
              title={course.title}
              summary={course.summary || course.description}
              badges={
                <>
                  {renderCategoryBadge(course)}
                  {isAdmin ? (
                    <Badge variant='outline'>
                      {t(
                        getOptionLabelKey(
                          PUBLISH_STATUS_OPTIONS,
                          course.status
                        ) ?? 'Draft'
                      )}
                    </Badge>
                  ) : null}
                </>
              }
              meta={[
                {
                  label: t('Capacity'),
                  value:
                    course.capacity > 0
                      ? course.capacity.toLocaleString()
                      : t('Unlimited'),
                },
                {
                  label: t('Price'),
                  value:
                    course.price_mode === PRICE_MODE.PAID
                      ? course.price.toLocaleString()
                      : t('Free'),
                },
                {
                  label: t('Created'),
                  value: course.created_at
                    ? dayjs(course.created_at).format('YYYY-MM-DD')
                    : '-',
                },
              ]}
              actions={renderActions(course)}
            />
          ))}
        </div>
      )}

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
