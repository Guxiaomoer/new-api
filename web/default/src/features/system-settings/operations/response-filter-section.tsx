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
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import i18next from 'i18next'
import { toast } from 'sonner'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
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

const responseFilterSchema = z.object({
  'general_setting.upstream_pollution_keywords': z.string(),
  'general_setting.upstream_pollution_disable_channel': z.boolean(),
  'general_setting.upstream_pollution_json_template': z.string(),
  'general_setting.upstream_pollution_stream_template': z.string(),
  'general_setting.upstream_failure_json_template': z.string(),
  'general_setting.upstream_failure_stream_template': z.string(),
})

type ResponseFilterFormValues = z.infer<typeof responseFilterSchema>

type ResponseFilterSectionProps = {
  defaultValues: ResponseFilterFormValues
}

const serializeValue = (value: unknown): string => {
  if (typeof value === 'boolean') return String(value)
  return String(value ?? '')
}

export function ResponseFilterSection({
  defaultValues,
}: ResponseFilterSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<ResponseFilterFormValues>({
    resolver: zodResolver(responseFilterSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (values: ResponseFilterFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof ResponseFilterFormValues]
    )

    if (updates.length === 0) {
      toast.info(i18next.t('No changes to save'))
      return
    }

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: serializeValue(value) })
    }
  }

  const isDirty = form.formState.isDirty
  const isSubmitting = form.formState.isSubmitting
  const handleSubmit = form.handleSubmit(onSubmit)

  return (
    <SettingsSection title={t('Upstream Response Filter')}>
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
                name='general_setting.upstream_pollution_disable_channel'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Auto-disable channel on upstream pollution')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, a response matching pollution keywords will disable the corresponding channel so you can replace the key manually.'
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
                name='general_setting.upstream_pollution_keywords'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Upstream pollution keywords')}</FormLabel>
                    <FormControl>
                      <Textarea
                        rows={6}
                        placeholder={t(
                          'One keyword per line. Configure exact phrases that indicate polluted upstream responses.'
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
                        'If any keyword is found in the upstream response, the response is blocked, logged, and handled according to the auto-disable switch.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='general_setting.upstream_pollution_json_template'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Pollution block non-stream response template')}
                    </FormLabel>
                    <FormControl>
                      <Textarea
                        rows={8}
                        placeholder={t(
                          'Optional JSON template. Leave empty to use the built-in blocked-error response. Variables: {{.Model}} {{.Keyword}} {{.ChannelId}} {{.ChannelName}} {{.RequestId}} {{.Created}} {{.Timestamp}}. Use {{json .Model}} inside JSON strings.'
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
                        'When a non-stream response matches a keyword, the rendered template is returned as HTTP 200 application/json. Template errors or invalid JSON fall back to the built-in blocked-error response.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='general_setting.upstream_pollution_stream_template'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Pollution block stream response template')}
                    </FormLabel>
                    <FormControl>
                      <Textarea
                        rows={8}
                        placeholder={t(
                          'Optional SSE template. Leave empty to use the built-in blocked-error SSE frame. Example:\ndata: {"choices":[{"delta":{"content":"Blocked by policy"}}]}\n\ndata: [DONE]\n\n'
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
                        'For stream requests, include complete SSE frames yourself. The rendered template is returned as HTTP 200 text/event-stream without further modification.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='general_setting.upstream_failure_json_template'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Upstream failure non-stream response template')}
                    </FormLabel>
                    <FormControl>
                      <Textarea
                        rows={8}
                        placeholder={t(
                          'Optional JSON template. Leave empty to keep the original error response. Example: {"error":{"message":"Service is temporarily unavailable","type":"upstream_maintenance","code":"upstream_maintenance"}}'
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
                        'Only applies after upstream/channel retries are exhausted. Local authentication, quota, invalid request, and sensitive-word errors are not rewritten. Variables: {{.Model}} {{.ErrorCode}} {{.StatusCode}} {{.ChannelId}} {{.ChannelName}} {{.RequestId}} {{.Created}} {{.Timestamp}}.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='general_setting.upstream_failure_stream_template'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Upstream failure stream response template')}
                    </FormLabel>
                    <FormControl>
                      <Textarea
                        rows={8}
                        placeholder={t(
                          'Optional SSE template. Leave empty to keep the original error response. Example:\ndata: {"choices":[{"delta":{"content":"Service is temporarily unavailable"}}]}\n\ndata: [DONE]\n\n'
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
                        'When an upstream/channel failure happens for a stream request, the rendered template is returned as HTTP 200 text/event-stream. Admin logs still keep the real error.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
