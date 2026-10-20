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

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { testTrainingService } from '@/features/model-training/api'
import { showTrainingError } from '@/features/model-training/lib/errors'

const schema = z.object({
  TrainingServiceURL: z.string(),
  TrainingServiceLLMBaseURL: z.string(),
  TrainingServiceSecret: z.string(),
  TrainingServiceTimeoutSeconds: z.string(),
  TrainingServiceTokenTTLHours: z.string(),
  TrainingServiceDemoMode: z.boolean(),
})

type Values = z.input<typeof schema>

type TestState = {
  loading: boolean
  ok: boolean | null
  message: string | null
}

export function TrainingServiceSettingsSection(props: {
  defaultValues: {
    TrainingServiceURL: string
    TrainingServiceLLMBaseURL: string
    TrainingServiceTimeoutSeconds: number
    TrainingServiceTokenTTLHours: number
    TrainingServiceDemoMode: boolean
  }
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
      TrainingServiceURL: props.defaultValues.TrainingServiceURL ?? '',
      TrainingServiceLLMBaseURL:
        props.defaultValues.TrainingServiceLLMBaseURL ?? '',
      TrainingServiceSecret: '',
      TrainingServiceTimeoutSeconds: String(
        props.defaultValues.TrainingServiceTimeoutSeconds ?? 60
      ),
      TrainingServiceTokenTTLHours: String(
        props.defaultValues.TrainingServiceTokenTTLHours ?? 168
      ),
      TrainingServiceDemoMode: props.defaultValues.TrainingServiceDemoMode ?? false,
    },
  })

  const { isDirty, isSubmitting } = form.formState

  const onSubmit = async (values: Values) => {
    const updates: Array<{ key: string; value: string }> = []
    const fields = [
      'TrainingServiceURL',
      'TrainingServiceLLMBaseURL',
      'TrainingServiceTimeoutSeconds',
      'TrainingServiceTokenTTLHours',
    ] as const
    for (const key of fields) {
      const next = String(values[key] ?? '').trim()
      const current = String(
        props.defaultValues[key as keyof typeof props.defaultValues] ?? ''
      ).trim()
      if (next !== current) {
        updates.push({ key, value: next })
      }
    }
    // Demo mode is a checkbox, so a toggle is always an explicit change.
    const demoMode = values.TrainingServiceDemoMode === true
    if (demoMode !== props.defaultValues.TrainingServiceDemoMode) {
      updates.push({
        key: 'TrainingServiceDemoMode',
        value: demoMode ? 'true' : 'false',
      })
    }
    // The shared secret is write-only: an empty field keeps the saved value.
    const secret = values.TrainingServiceSecret.trim()
    if (secret) {
      updates.push({ key: 'TrainingServiceSecret', value: secret })
    }
    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
    form.reset({ ...values, TrainingServiceSecret: '' })
  }

  const handleTest = async () => {
    setTestState({ loading: true, ok: null, message: null })
    try {
      const result = await testTrainingService()
      if (result.health?.demo_mode) {
        setTestState({
          loading: false,
          ok: true,
          message: t(
            'Connected in demo mode: preparation and training are simulated, no GPU is used.'
          ),
        })
        return
      }
      const versions = result.health?.versions ?? {}
      const gpus = result.health?.gpus ?? []
      setTestState({
        loading: false,
        ok: true,
        message: t(
          'Connected. DataFlow {{dataflow}}, LLaMA Factory {{llamafactory}}, {{gpus}} GPU(s).',
          {
            dataflow: versions.dataflow || t('unknown'),
            llamafactory: versions.llamafactory || t('unknown'),
            gpus: gpus.length,
          }
        ),
      })
    } catch (error) {
      setTestState({
        loading: false,
        ok: false,
        message:
          error instanceof Error ? error.message : t('Connection failed'),
      })
      showTrainingError(error)
    }
  }

  return (
    <SettingsSection title={t('Model Training Service')}>
      <Form {...form}>
        <SettingsForm
          onSubmit={form.handleSubmit(onSubmit)}
          autoComplete='off'
        >
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save model training settings'
          />
          <FormField
            control={form.control}
            name='TrainingServiceDemoMode'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-3'>
                <div className='space-y-0.5'>
                  <FormLabel>{t('Demo mode (no GPU)')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Run the whole flow inside the gateway with simulated preparation and training. No URL, secret, GPU or LLaMA Factory installation is needed.'
                    )}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='TrainingServiceURL'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Training service URL')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder='https://gpu-host:8083'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Address of the training service running on the GPU host. The integration is enabled once this and the secret are set.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='TrainingServiceSecret'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Training service secret')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='new-password'
                    type='password'
                    placeholder={t('Leave empty to keep the saved secret')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Shared bearer token configured on the training host. It is write-only and never returned to the console.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='TrainingServiceLLMBaseURL'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Data cleaning gateway URL')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder='http://newapi:3000/v1'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'OpenAI-compatible endpoint the training service calls for DataFlow cleaning. It must be reachable from the GPU host.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <SettingsSwitchContent>
            <SettingsSwitchItem>
              <FormField
                control={form.control}
                name='TrainingServiceTimeoutSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Request timeout (seconds)')}</FormLabel>
                    <FormControl>
                      <Input type='number' min={1} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsSwitchItem>
            <SettingsSwitchItem>
              <FormField
                control={form.control}
                name='TrainingServiceTokenTTLHours'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Data cleaning key TTL (hours)')}</FormLabel>
                    <FormControl>
                      <Input type='number' min={1} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Lifetime of the temporary key issued to the training service for each preparation run.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsSwitchItem>
          </SettingsSwitchContent>

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

          {testState.ok === false ? (
            <Alert variant='destructive'>
              <AlertTitle>{t('Connection failed')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Check that the service is reachable from the gateway and that the secret matches.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
