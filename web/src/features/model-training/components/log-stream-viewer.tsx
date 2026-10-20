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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'

import { streamLogs } from '../api'

/** Upper bound on retained lines so a chatty training run cannot exhaust memory. */
const MAX_LOG_LINES = 2000

type LogStreamViewerProps = {
  /** Console endpoint that proxies the training service stream. */
  path: string
}

/**
 * Streams a log endpoint for as long as it is mounted. Callers render it inside
 * a dialog or sheet, which unmounts its content on close and therefore closes
 * the stream through the effect cleanup.
 */
export function LogStreamViewer(props: LogStreamViewerProps) {
  const { t } = useTranslation()
  const [lines, setLines] = useState<string[]>([])
  const [status, setStatus] = useState<'streaming' | 'done' | 'error'>(
    'streaming'
  )
  const [error, setError] = useState('')
  const offsetRef = useRef(0)
  const bottomRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!props.path) return
    const controller = new AbortController()

    void streamLogs(
      props.path,
      offsetRef.current,
      {
        onFrame: (frame) => {
          offsetRef.current = frame.offset
          setLines((previous) => {
            const next = [...previous, frame.line]
            return next.length > MAX_LOG_LINES
              ? next.slice(next.length - MAX_LOG_LINES)
              : next
          })
        },
        onDone: () => setStatus('done'),
        onError: (message) => {
          setError(message)
          setStatus('error')
        },
      },
      controller.signal
    )

    return () => controller.abort()
  }, [props.path])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: 'end' })
  }, [lines])

  const clear = () => {
    setLines([])
    offsetRef.current = 0
  }

  return (
    <div className='space-y-2'>
      <div className='text-muted-foreground flex items-center justify-between text-xs'>
        <span aria-live='polite'>
          {status === 'streaming' ? t('Streaming') : null}
          {status === 'done' ? t('Stream finished') : null}
          {status === 'error' ? t('Stream failed') : null}
        </span>
        <Button type='button' variant='ghost' size='sm' onClick={clear}>
          {t('Clear')}
        </Button>
      </div>
      <ScrollArea className='bg-muted/40 h-72 rounded-md border'>
        <pre className='p-3 text-xs whitespace-pre-wrap break-all'>
          {lines.length === 0 ? t('No log output yet') : lines.join('\n')}
        </pre>
        <div ref={bottomRef} />
      </ScrollArea>
      {error ? <p className='text-destructive text-xs'>{error}</p> : null}
    </div>
  )
}
