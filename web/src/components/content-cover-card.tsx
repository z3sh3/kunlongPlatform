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
import type { ReactNode } from 'react'

import { Card } from '@/components/ui/card'
import { cn } from '@/lib/utils'

export type ContentCoverCardMeta = {
  label: string
  value: string
}

type ContentCoverCardProps = {
  /** Cover image address; a gradient placeholder is shown when absent. */
  coverUrl?: string
  title: string
  /** Short introduction shown under the title. */
  summary?: string
  /** Badges rendered over the cover (category, type, status). */
  badges?: ReactNode
  meta?: ContentCoverCardMeta[]
  /** Card actions (enroll, apply, edit, publish, delete). */
  actions?: ReactNode
  className?: string
}

/** First characters of the title, used by the placeholder cover. */
function coverInitials(title: string): string {
  const trimmed = title.trim()
  if (!trimmed) return '?'
  return [...trimmed].slice(0, 2).join('')
}

/**
 * Card used by the training-certification and incubation-program catalogues.
 *
 * Both modules present the same shape (cover, title, summary, meta, actions), so
 * the layout lives here instead of being duplicated per feature.
 */
export function ContentCoverCard(props: ContentCoverCardProps) {
  return (
    <Card className={cn('h-full gap-3 pt-0', props.className)}>
      <div className='bg-muted relative aspect-[16/9] w-full overflow-hidden'>
        {props.coverUrl ? (
          <img
            src={props.coverUrl}
            alt=''
            loading='lazy'
            className='h-full w-full object-cover'
          />
        ) : (
          <div
            aria-hidden='true'
            className='from-primary/20 to-primary/5 text-primary/70 flex h-full w-full items-center justify-center bg-gradient-to-br text-2xl font-semibold'
          >
            {coverInitials(props.title)}
          </div>
        )}
        {props.badges ? (
          <div className='absolute start-3 top-3 flex flex-wrap gap-1'>
            {props.badges}
          </div>
        ) : null}
      </div>

      <div className='flex min-h-0 flex-1 flex-col gap-2 px-4'>
        <h3 className='line-clamp-2 text-sm font-semibold'>{props.title}</h3>
        {props.summary ? (
          <p className='text-muted-foreground line-clamp-2 text-xs'>
            {props.summary}
          </p>
        ) : null}
        {props.meta?.length ? (
          <dl className='text-muted-foreground mt-auto grid grid-cols-2 gap-x-3 gap-y-1 text-xs'>
            {props.meta.map((item) => (
              <div key={item.label} className='flex items-center gap-1'>
                <dt className='shrink-0'>{item.label}:</dt>
                <dd className='truncate'>{item.value}</dd>
              </div>
            ))}
          </dl>
        ) : null}
      </div>

      {props.actions ? (
        <div className='flex flex-wrap items-center justify-end gap-1 border-t px-4 pt-3'>
          {props.actions}
        </div>
      ) : null}
    </Card>
  )
}
