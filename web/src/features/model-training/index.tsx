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

import { JobsList } from './components/jobs-list'

export function ModelTraining() {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const isAdmin = (user?.role ?? 0) >= ROLE.ADMIN

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('Model Training')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Tabs defaultValue='mine'>
          <TabsList className='flex-wrap'>
            <TabsTrigger value='mine'>{t('My jobs')}</TabsTrigger>
            {isAdmin ? (
              <TabsTrigger value='admin'>{t('All jobs')}</TabsTrigger>
            ) : null}
          </TabsList>
          <TabsContent value='mine' className='mt-4'>
            <JobsList mode='mine' />
          </TabsContent>
          {isAdmin ? (
            <TabsContent value='admin' className='mt-4'>
              <JobsList mode='admin' />
            </TabsContent>
          ) : null}
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
