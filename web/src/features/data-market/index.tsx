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
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { AdminGrantsList } from './components/admin-grants-list'
import { DatasetsList } from './components/datasets-list'
import { GrantsList } from './components/grants-list'

export function DataMarket() {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const isAdmin = (user?.role ?? 0) >= ROLE.ADMIN

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
          <TabsContent value='browse' className='mt-4'>
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
