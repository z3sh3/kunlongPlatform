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
  adminDeleteProgram,
  adminListPrograms,
  adminSeedDemoPrograms,
  adminUpdateProgramStatus,
  listPrograms,
} from '../api'
import {
  PROGRAM_TYPE,
  PUBLISH_STATUS,
  PUBLISH_STATUS_OPTIONS,
  getOptionLabelKey,
} from '../constants'
import type { IncubationProgram } from '../types'
import { ApplicationFormDialog } from './application-form-dialog'
import { ProgramFormDialog } from './program-form-dialog'

type ProgramsListMode = 'browse' | 'admin'

type ProgramsListProps = {
  mode: ProgramsListMode
}

export function ProgramsList(props: ProgramsListProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<IncubationProgram | null>(null)
  const [applyProgramId, setApplyProgramId] = useState<number | undefined>(
    undefined
  )
  const [applyOpen, setApplyOpen] = useState(false)

  const isAdmin = props.mode === 'admin'

  const programsQuery = useQuery({
    queryKey: [
      'enterprise-incubation',
      'programs',
      props.mode,
      page,
      pageSize,
    ],
    queryFn: () =>
      isAdmin
        ? adminListPrograms({ p: page, page_size: pageSize })
        : listPrograms({ p: page, page_size: pageSize }),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({
      queryKey: ['enterprise-incubation', 'programs'],
    })
  }

  const deleteMutation = useMutation({
    mutationFn: (id: number) => adminDeleteProgram(id),
    onSuccess: () => {
      toast.success(t('Program deleted'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  const statusMutation = useMutation({
    mutationFn: (values: { id: number; status: number }) =>
      adminUpdateProgramStatus(values.id, values.status),
    onSuccess: () => {
      toast.success(t('Program status updated'))
      invalidate()
    },
    onError: (error) => handleServerError(error),
  })

  // Publishes the three bundled demo cases (program + one application per
  // review stage). Idempotent: existing records are reported, not duplicated.
  const seedDemoMutation = useMutation({
    mutationFn: () => adminSeedDemoPrograms(),
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
        queryKey: ['enterprise-incubation', 'applications'],
      })
    },
    onError: (error) => handleServerError(error),
  })

  const renderTypeBadge = (program: IncubationProgram) => {
    let label: string
    if (program.type === PROGRAM_TYPE.INCUBATION) {
      label = t('Enterprise incubation')
    } else if (program.type === PROGRAM_TYPE.SME_SUPPORT) {
      label = t('SME support')
    } else {
      label = t('Government & park programs')
    }
    return <Badge variant='outline'>{label}</Badge>
  }

  const renderAdminStatusButton = (program: IncubationProgram) => {
    const isPublished = program.status === PUBLISH_STATUS.PUBLISHED
    return (
      <Button
        variant='ghost'
        size='sm'
        onClick={() =>
          statusMutation.mutate({
            id: program.id,
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

  const renderActions = (program: IncubationProgram) => {
    if (!isAdmin) {
      return (
        <Button
          size='sm'
          onClick={() => {
            setApplyProgramId(program.id)
            setApplyOpen(true)
          }}
        >
          {t('Apply')}
        </Button>
      )
    }
    return (
      <>
        <Button
          variant='ghost'
          size='sm'
          onClick={() => {
            setEditing(program)
            setFormOpen(true)
          }}
        >
          {t('Edit')}
        </Button>
        {renderAdminStatusButton(program)}
        <Button
          variant='ghost'
          size='sm'
          className='text-destructive hover:text-destructive'
          onClick={() => deleteMutation.mutate(program.id)}
          disabled={deleteMutation.isPending}
        >
          {t('Delete')}
        </Button>
      </>
    )
  }

  const items = programsQuery.data?.items ?? []
  const isEmpty = programsQuery.isSuccess && items.length === 0

  return (
    <div className='space-y-4'>
      {isAdmin ? (
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Demo cases cover incubation, SME support and government park programs with applications at each review stage.'
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
              {t('Create program')}
            </Button>
          </div>
        </div>
      ) : null}

      {isEmpty ? (
        <p className='text-muted-foreground py-16 text-center text-sm'>
          {t('No programs yet')}
        </p>
      ) : (
        <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
          {items.map((program) => (
            <ContentCoverCard
              key={program.id}
              coverUrl={program.cover_url}
              title={program.title}
              summary={program.summary || program.description}
              badges={
                <>
                  {renderTypeBadge(program)}
                  {isAdmin ? (
                    <Badge variant='outline'>
                      {t(
                        getOptionLabelKey(
                          PUBLISH_STATUS_OPTIONS,
                          program.status
                        ) ?? 'Draft'
                      )}
                    </Badge>
                  ) : null}
                </>
              }
              meta={[
                {
                  label: t('Created'),
                  value: program.created_at
                    ? dayjs(program.created_at).format('YYYY-MM-DD')
                    : '-',
                },
              ]}
              actions={renderActions(program)}
            />
          ))}
        </div>
      )}

      {programsQuery.data ? (
        <ServerPagination
          page={page}
          pageSize={pageSize}
          total={programsQuery.data.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          disabled={programsQuery.isFetching}
        />
      ) : null}

      <ProgramFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        program={editing}
      />
      <ApplicationFormDialog
        open={applyOpen}
        onOpenChange={setApplyOpen}
        programId={applyProgramId}
      />
    </div>
  )
}
