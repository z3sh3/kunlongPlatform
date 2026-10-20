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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'
import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { AdminGrantsList } from './components/admin-grants-list'
import { DatasetsList } from './components/datasets-list'
import { GrantsList } from './components/grants-list'

export function DataMarket() {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const isAdmin = (user?.role ?? 0) >= ROLE.ADMIN
  const queryClient = useQueryClient()

  // Publishes the three bundled demonstration listings (domain knowledge, code,
  // reasoning). Idempotent: existing listings are reported, not duplicated.
  const seedMutation = useMutation({
    mutationFn: async () => {
      const res = await api.post('/api/data-market/admin/demo-datasets')
      const payload = res.data as {
        success: boolean
        message?: string
        data?: { created?: unknown[]; existing?: unknown[] }
      }
      if (!payload?.success) {
        throw new Error(payload?.message || 'Request failed')
      }
      return {
        created: payload.data?.created?.length ?? 0,
        existing: payload.data?.existing?.length ?? 0,
      }
    },
    onSuccess: (result) => {
      if (result.created > 0) {
        toast.success(
          t('Published {{count}} demo dataset(s)', { count: result.created })
        )
      } else {
        toast.info(t('Demo datasets already exist'))
      }
      queryClient.invalidateQueries({ queryKey: ['data-market'] })
    },
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : t('Request failed')),
  })

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Data Market')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Tabs defaultValue='browse'>
          <TabsList className='flex-wrap'>
            <TabsTrigger value='browse'>{t('Browse datasets')}</TabsTrigger>
            <TabsTrigger value='mine'>{t('My datasets')}</TabsTrigger>
            <TabsTrigger value='grants'>{t('My access')}</TabsTrigger>
            {isAdmin ? (
              <TabsTrigger value='admin'>{t('Administration')}</TabsTrigger>
            ) : null}
          </TabsList>
          <TabsContent value='browse' className='mt-4 space-y-3'>
            {isAdmin ? (
              <div className='flex flex-wrap items-center justify-between gap-2 rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Demo datasets cover three dimensions: domain knowledge, code and reasoning.'
                  )}
                </p>
                <Button
                  variant='outline'
                  disabled={seedMutation.isPending}
                  onClick={() => seedMutation.mutate()}
                >
                  {t('Generate demo datasets')}
                </Button>
              </div>
            ) : null}
            <DatasetsList mode='browse' />
          </TabsContent>
          <TabsContent value='mine' className='mt-4'>
            <DatasetsList mode='mine' />
          </TabsContent>
          <TabsContent value='grants' className='mt-4'>
            <GrantsList />
          </TabsContent>
          {isAdmin ? (
            <TabsContent value='admin' className='mt-4 space-y-8'>
              <div className='space-y-2'>
                <h3 className='text-sm font-medium'>{t('Datasets')}</h3>
                <DatasetsList mode='admin' />
              </div>
              <div className='space-y-2'>
                <h3 className='text-sm font-medium'>
                  {t('Access applications')}
                </h3>
                <AdminGrantsList />
              </div>
            </TabsContent>
          ) : null}
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
