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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { api } from '@/lib/api'

import { createDatasetFromURL, uploadDataset } from '../api'
import { MAX_DATASET_UPLOAD_BYTES } from '../constants'
import { showTrainingError } from '../lib/errors'

type DatasetFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type MarketDataset = {
  id: number
  name: string
  file_url: string
}

/** Loads the data market listings the user may reuse as training input. */
async function fetchMarketDatasets(): Promise<MarketDataset[]> {
  const res = await api.get('/api/data-market/datasets?p=1&page_size=50')
  const payload = res.data as {
    success: boolean
    data?: { items?: MarketDataset[] }
  }
  return payload?.data?.items ?? []
}

export function DatasetFormDialog(props: DatasetFormDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [mode, setMode] = useState<'upload' | 'url' | 'market'>('upload')
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [marketId, setMarketId] = useState('')

  const marketQuery = useQuery({
    queryKey: ['data-market', 'datasets', 'for-training'],
    queryFn: fetchMarketDatasets,
    enabled: props.open && mode === 'market',
  })

  const mutation = useMutation({
    mutationFn: async () => {
      if (mode === 'upload') {
        if (!file) throw new Error(t('Choose a dataset file first'))
        if (file.size > MAX_DATASET_UPLOAD_BYTES) {
          throw new Error(t('The dataset exceeds the 200 MB limit'))
        }
        await uploadDataset(file, name || file.name)
        return
      }
      if (mode === 'market') {
        const selected = (marketQuery.data ?? []).find(
          (item) => String(item.id) === marketId
        )
        if (!selected?.file_url) {
          throw new Error(t('Select a data market dataset first'))
        }
        await createDatasetFromURL({
          url: selected.file_url,
          name: name || selected.name,
          market_dataset_id: selected.id,
        })
        return
      }
      if (!url.trim()) throw new Error(t('Enter a dataset URL'))
      await createDatasetFromURL({ url: url.trim(), name: name || undefined })
    },
    onSuccess: () => {
      toast.success(t('Dataset added'))
      queryClient.invalidateQueries({ queryKey: ['model-training', 'datasets'] })
      setName('')
      setUrl('')
      setFile(null)
      setMarketId('')
      props.onOpenChange(false)
    },
    onError: (error) =>
      showTrainingError(
        error instanceof Error && !('isAxiosError' in error)
          ? error
          : error
      ),
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('Add dataset')}</DialogTitle>
          <DialogDescription>
            {t(
              'The file is uploaded to the training host. Uploaded data can then be prepared with DataFlow.'
            )}
          </DialogDescription>
        </DialogHeader>

        <Tabs value={mode} onValueChange={(value) => setMode(value as typeof mode)}>
          <TabsList className='flex-wrap'>
            <TabsTrigger value='upload'>{t('Upload')}</TabsTrigger>
            <TabsTrigger value='url'>{t('URL')}</TabsTrigger>
            <TabsTrigger value='market'>{t('Data market')}</TabsTrigger>
          </TabsList>

          <TabsContent value='upload' className='mt-4 space-y-2'>
            <Label htmlFor='dataset-file'>{t('Dataset file')}</Label>
            <Input
              id='dataset-file'
              type='file'
              accept='.jsonl,.json,.csv,.parquet,.txt,.zip,.pdf'
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
            />
            <p className='text-muted-foreground text-xs'>
              {t('JSONL files train directly; PDFs and text are converted by DataFlow.')}
            </p>
          </TabsContent>

          <TabsContent value='url' className='mt-4 space-y-2'>
            <Label htmlFor='dataset-url'>{t('Dataset URL')}</Label>
            <Input
              id='dataset-url'
              value={url}
              onChange={(event) => setUrl(event.target.value)}
              placeholder='https://example.com/data.jsonl'
            />
          </TabsContent>

          <TabsContent value='market' className='mt-4 space-y-2'>
            <Label>{t('Data market dataset')}</Label>
            <Select
              value={marketId}
              onValueChange={(value) => setMarketId(value ?? '')}
              items={(marketQuery.data ?? []).map((item) => ({
                value: String(item.id),
                label: item.name,
              }))}
            >
              <SelectTrigger>
                <SelectValue placeholder={t('Select a dataset')} />
              </SelectTrigger>
              <SelectContent>
                {(marketQuery.data ?? []).map((item) => (
                  <SelectItem key={item.id} value={String(item.id)}>
                    {item.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {marketQuery.isSuccess && (marketQuery.data ?? []).length === 0 ? (
              <p className='text-muted-foreground text-xs'>
                {t('No published datasets are available in the data market.')}
              </p>
            ) : null}
          </TabsContent>
        </Tabs>

        <div className='space-y-2'>
          <Label htmlFor='dataset-name'>{t('Name')}</Label>
          <Input
            id='dataset-name'
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder={t('Optional display name')}
          />
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            disabled={mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {t('Add')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
