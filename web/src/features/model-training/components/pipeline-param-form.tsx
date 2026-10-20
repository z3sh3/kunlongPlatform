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

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import type { PipelineParam } from '../types'

type PipelineParamFormProps = {
  params: PipelineParam[]
  value: Record<string, unknown>
  onChange: (next: Record<string, unknown>) => void
}

/**
 * Renders the parameter form of a DataFlow pipeline template. The schema comes
 * from the training service, so a new template needs no frontend change.
 */
export function PipelineParamForm(props: PipelineParamFormProps) {
  const { t } = useTranslation()

  const set = (key: string, next: unknown) => {
    props.onChange({ ...props.value, [key]: next })
  }

  return (
    <div className='space-y-4'>
      {props.params.map((param) => {
        const label = t(param.label)
        const description = param.help ? t(param.help) : ''
        const current = props.value[param.key] ?? param.default ?? ''

        if (param.type === 'enum') {
          const options = param.options ?? []
          return (
            <div key={param.key} className='space-y-2'>
              <Label>{label}</Label>
              <Select
                value={String(current)}
                onValueChange={(value) => set(param.key, value ?? '')}
                items={options.map((option) => ({
                  value: option,
                  label: option,
                }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {options.map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {description ? (
                <p className='text-muted-foreground text-xs'>{description}</p>
              ) : null}
            </div>
          )
        }

        if (param.type === 'bool') {
          return (
            <div
              key={param.key}
              className='flex flex-row items-center justify-between rounded-lg border p-3'
            >
              <div className='space-y-0.5'>
                <Label>{label}</Label>
                {description ? (
                  <p className='text-muted-foreground text-xs'>{description}</p>
                ) : null}
              </div>
              <Switch
                checked={current === true}
                onCheckedChange={(checked) => set(param.key, checked)}
              />
            </div>
          )
        }

        if (param.type === 'text') {
          return (
            <div key={param.key} className='space-y-2'>
              <Label>{label}</Label>
              <Textarea
                value={String(current)}
                onChange={(event) => set(param.key, event.target.value)}
              />
              {description ? (
                <p className='text-muted-foreground text-xs'>{description}</p>
              ) : null}
            </div>
          )
        }

        if (param.type === 'int' || param.type === 'float') {
          return (
            <div key={param.key} className='space-y-2'>
              <Label>
                {label}
                {param.unit ? ` (${param.unit})` : ''}
              </Label>
              <Input
                type='number'
                step={param.type === 'float' ? '0.1' : '1'}
                min={param.minimum}
                max={param.maximum}
                value={String(current)}
                onChange={(event) => {
                  const parsed = Number(event.target.value)
                  set(param.key, Number.isFinite(parsed) ? parsed : 0)
                }}
              />
              {description ? (
                <p className='text-muted-foreground text-xs'>{description}</p>
              ) : null}
            </div>
          )
        }

        return (
          <div key={param.key} className='space-y-2'>
            <Label>{label}</Label>
            <Input
              value={String(current)}
              onChange={(event) => set(param.key, event.target.value)}
            />
            {description ? (
              <p className='text-muted-foreground text-xs'>{description}</p>
            ) : null}
          </div>
        )
      })}
    </div>
  )
}
