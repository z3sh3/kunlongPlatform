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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ServerPagination, StaticDataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  listArtifacts,
  promoteArtifactToModel,
  registerArtifactChannel,
} from '../api'
import { showTrainingError } from '../lib/errors'
import type { TrainedModel } from '../types'

/** Formats a byte count without pulling in a locale-specific formatter. */
function formatSize(bytes: number): string {
  if (bytes <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`
}

export function ArtifactsPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((s) => s.auth.user)
  const isAdmin = (user?.role ?? 0) >= ROLE.ADMIN
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [modelFor, setModelFor] = useState<TrainedModel | null>(null)
  const [channelFor, setChannelFor] = useState<TrainedModel | null>(null)
  const [description, setDescription] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [group, setGroup] = useState('default')

  const artifactsQuery = useQuery({
    queryKey: ['model-training', 'artifacts', page, pageSize],
    queryFn: () => listArtifacts({ p: page, page_size: pageSize }),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['model-training', 'artifacts'] })
  }

  const promoteMutation = useMutation({
    mutationFn: (payload: { id: number; description: string }) =>
      promoteArtifactToModel(payload.id, { description: payload.description }),
    onSuccess: () => {
      toast.success(t('Model added to the catalog'))
      setModelFor(null)
      setDescription('')
      invalidate()
    },
    onError: (error) => showTrainingError(error),
  })

  const channelMutation = useMutation({
    mutationFn: (payload: {
      id: number
      base_url: string
      key: string
      group: string
    }) =>
      registerArtifactChannel(payload.id, {
        base_url: payload.base_url,
        key: payload.key,
        group: payload.group,
      }),
    onSuccess: () => {
      toast.success(t('Inference channel registered'))
      setChannelFor(null)
      setBaseURL('')
      setApiKey('')
      invalidate()
    },
    onError: (error) => showTrainingError(error),
  })

  const columns = [
    {
      id: 'name',
      header: t('Name'),
      cell: (artifact: TrainedModel) => (
        <div className='min-w-0'>
          <p className='truncate font-medium'>{artifact.name}</p>
          <p className='text-muted-foreground truncate text-xs'>
            {artifact.base_model} · {artifact.method}
          </p>
        </div>
      ),
    },
    {
      id: 'kind',
      header: t('Artifact'),
      cell: (artifact: TrainedModel) => (
        <Badge variant='outline'>
          {artifact.artifact_kind || t('unknown')}
        </Badge>
      ),
    },
    {
      id: 'size',
      header: t('Size'),
      cell: (artifact: TrainedModel) => formatSize(artifact.size_bytes),
    },
    {
      id: 'channel',
      header: t('Channel'),
      cell: (artifact: TrainedModel) =>
        artifact.channel_id > 0 ? `#${artifact.channel_id}` : '-',
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cell: (artifact: TrainedModel) =>
        isAdmin ? (
          <div className='flex justify-end gap-1'>
            <Button
              variant='ghost'
              size='sm'
              disabled={artifact.model_catalog_id > 0}
              onClick={() => setModelFor(artifact)}
            >
              {artifact.model_catalog_id > 0
                ? t('In catalog')
                : t('Add to catalog')}
            </Button>
            <Button
              variant='ghost'
              size='sm'
              disabled={artifact.channel_id > 0}
              onClick={() => setChannelFor(artifact)}
            >
              {t('Register channel')}
            </Button>
          </div>
        ) : (
          <span className='text-muted-foreground text-xs'>
            {artifact.artifact_ref}
          </span>
        ),
    },
  ]

  const items = artifactsQuery.data?.items ?? []

  return (
    <div className='space-y-4'>
      <StaticDataTable
        columns={columns}
        data={items}
        getRowKey={(artifact) => artifact.id}
        empty={artifactsQuery.isSuccess && items.length === 0}
        emptyContent={t('No model assets yet')}
      />
      {artifactsQuery.data ? (
        <ServerPagination
          page={page}
          pageSize={pageSize}
          total={artifactsQuery.data.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          disabled={artifactsQuery.isFetching}
        />
      ) : null}

      <Dialog
        open={modelFor !== null}
        onOpenChange={(open) => {
          if (!open) setModelFor(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Add to model catalog')}</DialogTitle>
            <DialogDescription>
              {t(
                'The trained model is listed in the model square. Register a channel to serve it.'
              )}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-2'>
            <Label htmlFor='catalog-description'>{t('Description')}</Label>
            <Input
              id='catalog-description'
              value={description}
              onChange={(event) => setDescription(event.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setModelFor(null)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={promoteMutation.isPending}
              onClick={() =>
                modelFor &&
                promoteMutation.mutate({
                  id: modelFor.id,
                  description,
                })
              }
            >
              {t('Add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={channelFor !== null}
        onOpenChange={(open) => {
          if (!open) setChannelFor(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Register inference channel')}</DialogTitle>
            <DialogDescription>
              {t(
                'Point the channel at an OpenAI-compatible server that serves this artifact.'
              )}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-3'>
            <div className='space-y-2'>
              <Label htmlFor='channel-base-url'>{t('Base URL')}</Label>
              <Input
                id='channel-base-url'
                value={baseURL}
                onChange={(event) => setBaseURL(event.target.value)}
                placeholder='http://gpu-host:8000/v1'
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='channel-key'>{t('API key')}</Label>
              <Input
                id='channel-key'
                value={apiKey}
                onChange={(event) => setApiKey(event.target.value)}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='channel-group'>{t('Group')}</Label>
              <Input
                id='channel-group'
                value={group}
                onChange={(event) => setGroup(event.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setChannelFor(null)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={
                channelMutation.isPending || !baseURL.trim() || !apiKey.trim()
              }
              onClick={() =>
                channelFor &&
                channelMutation.mutate({
                  id: channelFor.id,
                  base_url: baseURL.trim(),
                  key: apiKey.trim(),
                  group: group.trim() || 'default',
                })
              }
            >
              {t('Register')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
