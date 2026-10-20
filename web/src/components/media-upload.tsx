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
import { Loader2, Upload } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'

type MediaUploadProps = {
  /** Current object URL, kept in the parent form field. */
  value: string
  onChange: (url: string) => void
  /** Object prefix, e.g. `talent`; the backend stores <module>/<yyyyMM>/<file>. */
  module: string
}

type UploadResponse = {
  success: boolean
  message?: string
  data?: { url?: string; key?: string }
}

/**
 * Uploads an image or media file to object storage and writes the resulting URL
 * into the field. The URL stays editable so an external asset can still be
 * pasted, and the upload button reports the backend reason when storage is not
 * configured yet.
 */
export function MediaUpload(props: MediaUploadProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement | null>(null)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')

  const upload = async (file: File) => {
    setUploading(true)
    setError('')
    try {
      const form = new FormData()
      form.append('file', file)
      form.append('module', props.module)
      const res = await api.post('/api/storage/upload', form)
      const payload = res.data as UploadResponse
      const url = payload?.data?.url
      if (!payload?.success || !url) {
        throw new Error(payload?.message || t('Upload failed'))
      }
      props.onChange(url)
    } catch (cause) {
      const message =
        cause instanceof Error ? cause.message : t('Upload failed')
      setError(message)
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className='space-y-2'>
      <div className='flex items-start gap-2'>
        <Input
          value={props.value}
          onChange={(event) => props.onChange(event.target.value)}
          placeholder='https://cdn.example.com/cover.webp'
        />
        <input
          ref={inputRef}
          type='file'
          accept='image/*,audio/*,video/*'
          className='hidden'
          onChange={(event) => {
            const file = event.target.files?.[0]
            event.target.value = ''
            if (file) void upload(file)
          }}
        />
        <Button
          type='button'
          variant='outline'
          disabled={uploading}
          onClick={() => inputRef.current?.click()}
        >
          {uploading ? <Loader2 className='animate-spin' /> : <Upload />}
          {t('Upload')}
        </Button>
      </div>
      {props.value ? (
        <img
          src={props.value}
          alt=''
          className='bg-muted h-24 w-40 rounded-md border object-cover'
        />
      ) : null}
      {error ? <p className='text-destructive text-xs'>{error}</p> : null}
    </div>
  )
}
