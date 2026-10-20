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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { DifyAgentsList } from './components/dify-agents-list'

export function AgentWorkshop() {
  const { t } = useTranslation()

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('Agent Workshop')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {/* Agents are created and maintained in Dify, which is the single
            source of truth; this page only reads them. */}
        <Tabs defaultValue='browse'>
          <TabsList className='flex-wrap'>
            <TabsTrigger value='browse'>{t('Browse agents')}</TabsTrigger>
            <TabsTrigger value='mine'>{t('My agents')}</TabsTrigger>
          </TabsList>
          <TabsContent value='browse' className='mt-4'>
            <DifyAgentsList mode='browse' />
          </TabsContent>
          <TabsContent value='mine' className='mt-4'>
            <DifyAgentsList mode='mine' />
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
