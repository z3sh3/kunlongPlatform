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
import { useQuery } from '@tanstack/react-query'
import { ExternalLink } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import { getDifyAgentAccess } from '../api'
import type { DifyAgent } from '../types'

type AgentAccessDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  agent: DifyAgent | null
  /** Public origin of the Dify console; joins console_path links. */
  consoleBaseURL: string
}

/** A full-width URL row with a copy button, shared by both access sections. */
function AccessUrlRow({ url }: { url: string }) {
  return (
    <div className='flex items-center gap-2'>
      <code className='bg-muted text-foreground min-w-0 flex-1 truncate rounded-md px-3 py-2 font-mono text-xs'>
        {url}
      </code>
      <CopyButton value={url} size='sm' />
    </div>
  )
}

/**
 * Explains the three ways to reach one agent: the published web app, the
 * service API, and the Dify console entry.
 */
export function AgentAccessDialog(props: AgentAccessDialogProps) {
  const { t } = useTranslation()

  const agent = props.agent
  const accessQuery = useQuery({
    queryKey: ['agent-workshop', 'dify-agent-access', agent?.id],
    queryFn: () => {
      if (agent === null) {
        return Promise.reject(new Error('no agent selected'))
      }
      return getDifyAgentAccess(agent.id)
    },
    enabled: props.open && agent !== null,
    retry: false,
  })

  const access = accessQuery.data
  const sectionTitle = 'text-sm font-medium'

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('How to use this agent')}</DialogTitle>
          <DialogDescription>{props.agent?.name}</DialogDescription>
        </DialogHeader>

        {accessQuery.isPending ? (
          <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
        ) : null}
        {accessQuery.isError ? (
          <p className='text-destructive text-sm'>
            {t('Failed to load agents from Dify')}
          </p>
        ) : null}

        {access ? (
          <div className='space-y-5'>
            <section className='space-y-2'>
              <h4 className={sectionTitle}>{t('Use in the web app')}</h4>
              {access.webapp_url ? (
                <AccessUrlRow url={access.webapp_url} />
              ) : (
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'Site access is not enabled for this app. Publish it and enable site access in Dify first.'
                  )}
                </p>
              )}
            </section>

            <section className='space-y-2'>
              <h4 className={sectionTitle}>{t('Call through the API')}</h4>
              {access.api_base_url ? (
                <div className='space-y-2'>
                  <AccessUrlRow url={access.api_base_url} />
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Create an API key in the Dify console access page, then send it as the Authorization: Bearer header.'
                    )}
                  </p>
                </div>
              ) : (
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'API access is not enabled. Enable it in the Dify console first.'
                  )}
                </p>
              )}
            </section>

            <section className='space-y-2'>
              <h4 className={sectionTitle}>
                {t('Orchestrate in the Dify console')}
              </h4>
              <Button
                variant='outline'
                size='sm'
                render={
                  <a
                    href={`${props.consoleBaseURL}${access.console_path}`}
                    target='_blank'
                    rel='noopener noreferrer'
                  />
                }
              >
                {t('Open')}
                <ExternalLink />
              </Button>
            </section>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
