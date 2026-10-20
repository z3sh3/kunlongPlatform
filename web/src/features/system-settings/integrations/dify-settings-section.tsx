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
along with this program. If not, see <http://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { CheckCircle2, Loader2, XCircle } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Alert, AlertDescription } from '@/components/ui/alert'
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
import {
  getServerErrorMessage,
  getServerMessage,
} from '@/lib/server-error-message'

import { testDifyConnection } from '../api'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const optionalHTTPURL = (t: (key: string) => string) =>
  z.string().refine((value) => {
    const trimmed = value.trim()
    return !trimmed || /^https?:\/\/\S+$/.test(trimmed)
  }, t('Enter an http(s) URL or leave blank'))

const createSchema = (t: (key: string) => string) =>
  z.object({
    DifyAPIBaseURL: optionalHTTPURL(t),
    DifyConsoleURL: optionalHTTPURL(t),
    DifyAccountEmail: z.string().refine((value) => {
      const trimmed = value.trim()
      return !trimmed || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)
    }, t('Enter a valid email or leave blank')),
    DifyAccountSecret: z.string(),
  })

type DifyFormValues = z.infer<ReturnType<typeof createSchema>>

type DifySettingsSectionProps = {
  defaultValues: DifyFormValues
}

/**
 * Connection settings of the Dify (agent development) console. Agents are
 * created in Dify, so the agent workshop reads them through this account.
 */
export function DifySettingsSection({
  defaultValues,
}: DifySettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const schema = createSchema(t)

  const form = useForm<DifyFormValues>({
    resolver: zodResolver(schema),
    defaultValues,
  })
  const { isDirty, isSubmitting } = form.formState
  useResetForm(form, defaultValues)

  const [testState, setTestState] = useState<{
    loading: boolean
    ok: boolean | null
    message: string | null
  }>({ loading: false, ok: null, message: null })

  const onSubmit = async (values: DifyFormValues) => {
    const updates: Array<{ key: string; value: string }> = []
    const fields = [
      'DifyAPIBaseURL',
      'DifyConsoleURL',
      'DifyAccountEmail',
    ] as const

    for (const key of fields) {
      const next = values[key].trim()
      if (next !== defaultValues[key].trim()) {
        updates.push({ key, value: next })
      }
    }
    // The password is write-only: an empty field keeps the saved value.
    const secret = values.DifyAccountSecret.trim()
    if (secret) {
      updates.push({ key: 'DifyAccountSecret', value: secret })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
    form.reset({ ...values, DifyAccountSecret: '' })
  }

  const handleTestConnection = async () => {
    const baseURL = form.getValues('DifyAPIBaseURL').trim()
    const email = form.getValues('DifyAccountEmail').trim()
    if (!baseURL || !email) {
      toast.error(
        t('Enter the Dify API Base URL and account email before testing')
      )
      return
    }
    setTestState({ loading: true, ok: null, message: null })
    try {
      const res = await testDifyConnection({
        // Test what is typed in the form so a fresh configuration works
        // before it is saved; a blank password falls back to the saved one.
        base_url: baseURL,
        email,
        password: form.getValues('DifyAccountSecret'),
      })
      if (res.success) {
        // The login and both app sources answered, but the public console
        // address may still be wrong (missing port, or another service).
        if (res.data?.console_url_ok === false) {
          setTestState({
            loading: false,
            ok: false,
            message: t(
              'Cannot reach the Dify console address. Check the Dify Console URL, including the port.'
            ),
          })
          return
        }
        setTestState({
          loading: false,
          ok: true,
          message: t('Found {{count}} agents in the Dify workspace', {
            count: res.data?.total ?? 0,
          }),
        })
        return
      }
      setTestState({
        loading: false,
        ok: false,
        message: getServerMessage(res) || t('Connection failed'),
      })
    } catch (error) {
      setTestState({
        loading: false,
        ok: false,
        message: getServerErrorMessage(error, t('Connection failed')),
      })
    }
  }

  return (
    <SettingsSection title={t('Dify Agent Service')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save Dify settings'
          />
          <FormField
            control={form.control}
            name='DifyAPIBaseURL'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Dify API Base URL')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder='http://dify-api:5001'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Internal address this server uses to call the Dify console API. Leave blank to disable the integration.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='DifyConsoleURL'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Dify Console URL')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder='http://192.168.1.10:8081'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Public address users open. Leave blank to derive it from the current host and the Dify console port.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='DifyAccountEmail'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Dify account email')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder='admin@example.com'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Use a dedicated Dify account that can see every agent of the workspace.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='DifyAccountSecret'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Dify account password')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    autoComplete='new-password'
                    placeholder={t('Leave blank to keep the current password')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Only used to obtain a console session; the password is stored with the other system options.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='flex flex-wrap items-center gap-2'>
            <Button
              type='button'
              variant='secondary'
              onClick={handleTestConnection}
              disabled={testState.loading || updateOption.isPending}
            >
              {testState.loading ? (
                <Loader2 className='me-2 size-4 animate-spin' />
              ) : null}
              {t('Test connection')}
            </Button>
            <span className='text-muted-foreground text-xs'>
              {t(
                'The connection test uses the values above; a blank password keeps the saved one'
              )}
            </span>
          </div>

          {testState.ok === null ? null : (
            <DifyConnectionAlert
              ok={testState.ok}
              message={testState.message}
            />
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

type DifyConnectionAlertProps = {
  ok: boolean
  message: string | null
}

function DifyConnectionAlert(props: DifyConnectionAlertProps) {
  const { t } = useTranslation()
  const fallback = props.ok ? t('Connection succeeded') : t('Connection failed')

  return (
    <Alert variant={props.ok ? 'default' : 'destructive'}>
      {props.ok ? <CheckCircle2 /> : <XCircle />}
      <AlertDescription>{props.message || fallback}</AlertDescription>
    </Alert>
  )
}
