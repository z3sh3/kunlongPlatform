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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'

import { createAgent, updateAgent } from '../api'
import { AGENT_FORM_DEFAULT_VALUES, agentFormSchema, type AgentFormValues } from '../lib/schema'
import type { Agent } from '../types'

type AgentFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** When set, the dialog edits this agent; otherwise it creates one. */
  agent?: Agent | null
}

export function AgentFormDialog(props: AgentFormDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const form = useForm<AgentFormValues>({
    resolver: zodResolver(agentFormSchema),
    defaultValues: AGENT_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    const agent = props.agent
    form.reset(
      agent
        ? {
            name: agent.name,
            description: agent.description,
            icon: agent.icon,
            category: agent.category,
            config: agent.config,
            price_mode: agent.price_mode,
            unit_price: agent.unit_price,
          }
        : AGENT_FORM_DEFAULT_VALUES
    )
  }, [props.agent, props.open, form])

  const saveMutation = useMutation({
    mutationFn: (values: AgentFormValues) => {
      if (props.agent) {
        return updateAgent(props.agent.id, values)
      }
      return createAgent(values).then(() => undefined)
    },
    onSuccess: () => {
      toast.success(
        props.agent
          ? t('Agent updated successfully')
          : t('Agent created successfully')
      )
      queryClient.invalidateQueries({ queryKey: ['agent-workshop', 'agents'] })
      props.onOpenChange(false)
    },
    onError: (error) => handleServerError(error),
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (!saveMutation.isPending) props.onOpenChange(open)
      }}
    >
      <DialogContent className='sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>
            {props.agent ? t('Edit agent') : t('Create agent')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Describe the agent and its orchestration config. Agents go through review before publication.'
            )}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
            className='space-y-4'
          >
            <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Agent name')}
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='category'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Category')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. assistant, coding')}
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Description')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('What does this agent do?')}
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='config'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Orchestration config')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('Low-code orchestration definition (JSON)')}
                      rows={4}
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='price_mode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Price mode')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        max={1}
                        {...field}
                        value={field.value ?? 0}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='unit_price'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Unit price')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        {...field}
                        value={field.value ?? 0}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <DialogFooter>
              <Button type='submit' disabled={saveMutation.isPending}>
                {props.agent ? t('Save changes') : t('Create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
