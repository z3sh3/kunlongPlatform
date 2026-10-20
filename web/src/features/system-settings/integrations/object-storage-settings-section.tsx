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
import { CheckCircle2, Loader2, XCircle } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
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
import { api } from '@/lib/api'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  ObjectStorageProvider: z.string(),
  ObjectStorageRegion: z.string(),
  ObjectStorageBucket: z.string(),
  ObjectStorageScheme: z.string(),
  ObjectStorageSecretId: z.string(),
  ObjectStorageSecretKey: z.string(),
  ObjectStoragePublicBaseURL: z.string(),
  ObjectStorageMaxMB: z.string(),
})

type Values = z.input<typeof schema>

type TestState = { loading: boolean; ok: boolean | null; message: string | null }

type ObjectStorageDefaults = {
  ObjectStorageProvider: string
  ObjectStorageRegion: string
  ObjectStorageBucket: string
  ObjectStorageScheme: string
  ObjectStorageSecretId: string
  ObjectStoragePublicBaseURL: string
  ObjectStorageMaxMB: number
}

/** Object storage configuration for uploaded media (Tencent COS). */
export function ObjectStorageSettingsSection(props: {
  defaultValues: ObjectStorageDefaults
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [testState, setTestState] = useState<TestState>({
    loading: false,
    ok: null,
    message: null,
  })

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      ObjectStorageProvider: props.defaultValues.ObjectStorageProvider ?? 'tencent',
      ObjectStorageRegion: props.defaultValues.ObjectStorageRegion ?? '',
      ObjectStorageBucket: props.defaultValues.ObjectStorageBucket ?? '',
      ObjectStorageScheme: props.defaultValues.ObjectStorageScheme ?? 'https',
      ObjectStorageSecretId: props.defaultValues.ObjectStorageSecretId ?? '',
      ObjectStorageSecretKey: '',
      ObjectStoragePublicBaseURL: props.defaultValues.ObjectStoragePublicBaseURL ?? '',
      ObjectStorageMaxMB: String(props.defaultValues.ObjectStorageMaxMB ?? 10),
    },
  })

  const { isDirty, isSubmitting } = form.formState

  const onSubmit = async (values: Values) => {
    const fields = [
      'ObjectStorageProvider',
      'ObjectStorageRegion',
      'ObjectStorageBucket',
      'ObjectStorageScheme',
      'ObjectStorageSecretId',
      'ObjectStoragePublicBaseURL',
      'ObjectStorageMaxMB',
    ] as const
    const updates: Array<{ key: string; value: string }> = []
    for (const key of fields) {
      const next = String(values[key] ?? '').trim()
      const current = String(
        props.defaultValues[key as keyof ObjectStorageDefaults] ?? ''
      ).trim()
      if (next !== current) {
        updates.push({ key, value: next })
      }
    }
    // The secret key is write-only: an empty field keeps the saved value.
    const secretKey = values.ObjectStorageSecretKey.trim()
    if (secretKey) {
      updates.push({ key: 'ObjectStorageSecretKey', value: secretKey })
    }
    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
    form.reset({ ...values, ObjectStorageSecretKey: '' })
  }

  const handleTest = async () => {
    setTestState({ loading: true, ok: null, message: null })
    try {
      const res = await api.post('/api/storage/admin/test')
      const payload = res.data as { success: boolean; message?: string; data?: { bucket?: string } }
      if (!payload?.success) {
        throw new Error(payload?.message || t('Connection failed'))
      }
      setTestState({
        loading: false,
        ok: true,
        message: t('Bucket {{bucket}} is reachable', {
          bucket: payload.data?.bucket ?? '',
        }),
      })
    } catch (error) {
      setTestState({
        loading: false,
        ok: false,
        message: error instanceof Error ? error.message : t('Connection failed'),
      })
    }
  }

  return (
    <SettingsSection title={t('Object Storage')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save object storage settings'
          />
          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='ObjectStorageProvider'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Provider')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='tencent' />
                  </FormControl>
                  <FormDescription>
                    {t('Only Tencent COS is supported.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='ObjectStorageRegion'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Region')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='ap-beijing' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='ObjectStorageBucket'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Bucket')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='mybucket-1250000000' />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Required before uploads work. Objects are stored as module/yyyyMM/file.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='ObjectStorageScheme'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Scheme')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='https' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='ObjectStorageSecretId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Secret ID')}</FormLabel>
                  <FormControl>
                    <Input {...field} autoComplete='off' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='ObjectStorageSecretKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Secret key')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type='password'
                      autoComplete='new-password'
                      placeholder={t('Leave empty to keep the saved secret')}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Write-only: it is never returned to the console.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='ObjectStoragePublicBaseURL'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Public base URL')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='https://cdn.example.com' />
                  </FormControl>
                  <FormDescription>
                    {t('Optional CDN or custom domain used in returned URLs.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='ObjectStorageMaxMB'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max upload size (MB)')}</FormLabel>
                  <FormControl>
                    <Input {...field} type='number' min={1} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='flex flex-wrap items-center gap-3'>
            <Button
              type='button'
              variant='outline'
              onClick={handleTest}
              disabled={testState.loading}
            >
              {testState.loading ? <Loader2 className='animate-spin' /> : null}
              {t('Test saved connection')}
            </Button>
            {testState.message ? (
              <div className='flex items-center gap-2 text-sm'>
                {testState.ok ? (
                  <CheckCircle2 className='text-primary size-4' />
                ) : (
                  <XCircle className='text-destructive size-4' />
                )}
                <span>{testState.message}</span>
              </div>
            ) : null}
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
