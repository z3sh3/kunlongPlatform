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

import { ApplicationsList } from './components/applications-list'
import { ProgramsList } from './components/programs-list'

export function EnterpriseIncubation() {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const isAdmin = (user?.role ?? 0) >= ROLE.ADMIN

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('Enterprise Incubation')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Tabs defaultValue='programs'>
          <TabsList className='flex-wrap'>
            <TabsTrigger value='programs'>{t('Programs')}</TabsTrigger>
            <TabsTrigger value='mine'>{t('My applications')}</TabsTrigger>
            {isAdmin ? (
              <TabsTrigger value='admin'>{t('Administration')}</TabsTrigger>
            ) : null}
          </TabsList>
          <TabsContent value='programs' className='mt-4'>
            <ProgramsList mode='browse' />
          </TabsContent>
          <TabsContent value='mine' className='mt-4'>
            <ApplicationsList mode='mine' />
          </TabsContent>
          {isAdmin ? (
            <TabsContent value='admin' className='mt-4 space-y-8'>
              <div className='space-y-2'>
                <h3 className='text-sm font-medium'>{t('Programs')}</h3>
                <ProgramsList mode='admin' />
              </div>
              <div className='space-y-2'>
                <h3 className='text-sm font-medium'>{t('Applications')}</h3>
                <ApplicationsList mode='admin' />
              </div>
            </TabsContent>
          ) : null}
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
