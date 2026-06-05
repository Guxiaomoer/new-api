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
import { useMemo, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import i18next from 'i18next'
import { toast } from 'sonner'
import { getChannels, updateChannel } from '@/features/channels/api'
import { channelsQueryKeys } from '@/features/channels/lib'
import type { Channel } from '@/features/channels/types'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const maintenanceResponseSchema = z.object({
  'general_setting.global_maintenance_enabled': z.boolean(),
  'general_setting.global_maintenance_message': z.string(),
})

type MaintenanceResponseFormValues = z.infer<typeof maintenanceResponseSchema>

type MaintenanceResponseSectionProps = {
  defaultValues: MaintenanceResponseFormValues
}

type ChannelMaintenanceSettings = {
  channel_maintenance_enabled?: boolean
  channel_maintenance_message?: string
}

const serializeValue = (value: unknown): string => {
  if (typeof value === 'boolean') return String(value)
  return String(value ?? '')
}

const parseChannelSettings = (settings: string): Record<string, unknown> => {
  if (!settings.trim()) return {}
  try {
    const parsed = JSON.parse(settings)
    if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    return {}
  }
  return {}
}

const getChannelMaintenanceSettings = (
  channel?: Channel
): ChannelMaintenanceSettings => {
  if (!channel) return {}
  const settings = parseChannelSettings(channel.settings || '{}')
  return {
    channel_maintenance_enabled:
      settings.channel_maintenance_enabled === true,
    channel_maintenance_message:
      typeof settings.channel_maintenance_message === 'string'
        ? settings.channel_maintenance_message
        : '',
  }
}

export function MaintenanceResponseSection({
  defaultValues,
}: MaintenanceResponseSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const queryClient = useQueryClient()
  const [selectedChannelId, setSelectedChannelId] = useState('')
  const [channelMaintenanceEnabled, setChannelMaintenanceEnabled] =
    useState(false)
  const [channelMaintenanceMessage, setChannelMaintenanceMessage] = useState('')

  const form = useForm<MaintenanceResponseFormValues>({
    resolver: zodResolver(maintenanceResponseSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const { data: channelsData, isLoading: isChannelsLoading } = useQuery({
    queryKey: channelsQueryKeys.list({ page_size: 200, id_sort: true }),
    queryFn: () => getChannels({ page_size: 200, id_sort: true }),
  })

  const channels = useMemo(
    () => channelsData?.data?.items ?? [],
    [channelsData?.data?.items]
  )

  const selectedChannel = useMemo(
    () => channels.find((channel) => String(channel.id) === selectedChannelId),
    [channels, selectedChannelId]
  )

  const channelMutation = useMutation({
    mutationFn: async () => {
      if (!selectedChannel) return null
      const settings = parseChannelSettings(selectedChannel.settings || '{}')
      settings.channel_maintenance_enabled = channelMaintenanceEnabled
      if (channelMaintenanceEnabled) {
        settings.channel_maintenance_message = channelMaintenanceMessage
      } else {
        delete settings.channel_maintenance_message
      }
      return updateChannel(selectedChannel.id, {
        settings: JSON.stringify(settings),
      })
    },
    onSuccess: async (response) => {
      if (response && !response.success) {
        toast.error(response.message || i18next.t('Save failed'))
        return
      }
      toast.success(i18next.t('Saved successfully'))
      await queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      if (selectedChannel) {
        await queryClient.invalidateQueries({
          queryKey: channelsQueryKeys.detail(selectedChannel.id),
        })
      }
    },
    onError: () => {
      toast.error(i18next.t('Save failed'))
    },
  })

  const onSubmit = async (values: MaintenanceResponseFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof MaintenanceResponseFormValues]
    )

    if (updates.length === 0) {
      form.reset(defaultValues)
      toast.info(i18next.t('No changes to save'))
      return
    }

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: serializeValue(value) })
    }
  }

  const handleChannelChange = (value: string | null) => {
    const nextValue = value ?? ''
    setSelectedChannelId(nextValue)
    const channel = channels.find((item) => String(item.id) === nextValue)
    const settings = getChannelMaintenanceSettings(channel)
    setChannelMaintenanceEnabled(settings.channel_maintenance_enabled === true)
    setChannelMaintenanceMessage(settings.channel_maintenance_message || '')
  }

  const isDirty = form.formState.isDirty
  const isSubmitting = form.formState.isSubmitting
  const handleSubmit = form.handleSubmit(onSubmit)

  return (
    <SettingsSection title={t('Maintenance Response')}>
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsFormGrid>
            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='general_setting.global_maintenance_enabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Enable global maintenance response')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, all relay requests return your configured HTTP 200 maintenance response before billing, channel selection, and upstream calls. OpenAI Realtime websocket is not affected.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='general_setting.global_maintenance_message'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default maintenance message')}</FormLabel>
                    <FormControl>
                      <Textarea
                        rows={3}
                        placeholder={t(
                          'Plain text shown when global/channel maintenance is active.'
                        )}
                        value={field.value ?? ''}
                        onChange={(event) => field.onChange(event.target.value)}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Enter plain text only. The backend automatically wraps it for Claude, OpenAI, Gemini, stream, and non-stream requests.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <div className='rounded-lg border border-border/60 bg-muted/10 p-4'>
                <div className='mb-4 space-y-1'>
                  <h3 className='text-sm font-medium'>
                    {t('Channel maintenance response')}
                  </h3>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Select a channel to return the maintenance message only when requests are routed to that channel. Other channels continue normally.'
                    )}
                  </p>
                </div>

                <div className='space-y-4'>
                  <FormItem>
                    <FormLabel>{t('Select channel')}</FormLabel>
                    <Select
                      value={selectedChannelId}
                      onValueChange={handleChannelChange}
                      disabled={isChannelsLoading || channelMutation.isPending}
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue
                          placeholder={
                            isChannelsLoading
                              ? t('Loading channels...')
                              : t('Choose a channel')
                          }
                        />
                      </SelectTrigger>
                      <SelectContent align='start'>
                        <SelectGroup>
                          {channels.map((channel) => (
                            <SelectItem
                              key={channel.id}
                              value={String(channel.id)}
                            >
                              #{channel.id} {channel.name}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'This saves into the selected channel settings. It does not enable global maintenance.'
                      )}
                    </FormDescription>
                  </FormItem>

                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Enable maintenance for this channel')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, requests routed to this channel return HTTP 200 with the maintenance message and will not call upstream.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <Switch
                      checked={channelMaintenanceEnabled}
                      onCheckedChange={setChannelMaintenanceEnabled}
                      disabled={!selectedChannel || channelMutation.isPending}
                    />
                  </SettingsSwitchItem>

                  <FormItem>
                    <FormLabel>{t('Maintenance message')}</FormLabel>
                    <Textarea
                      rows={3}
                      placeholder={t(
                        'Leave empty to use the default maintenance message above.'
                      )}
                      value={channelMaintenanceMessage}
                      onChange={(event) =>
                        setChannelMaintenanceMessage(event.target.value)
                      }
                      disabled={!selectedChannel || channelMutation.isPending}
                    />
                    <FormDescription>
                      {t(
                        'Enter plain text only. The backend automatically wraps it for Claude, OpenAI, Gemini, stream, and non-stream requests.'
                      )}
                    </FormDescription>
                  </FormItem>

                  <div className='flex justify-end'>
                    <Button
                      type='button'
                      onClick={() => channelMutation.mutate()}
                      disabled={!selectedChannel || channelMutation.isPending}
                    >
                      {channelMutation.isPending
                        ? t('Saving...')
                        : t('Save channel maintenance')}
                    </Button>
                  </div>
                </div>
              </div>
            </SettingsFormGridItem>
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
