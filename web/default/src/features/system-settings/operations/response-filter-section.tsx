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
  general_setting: z.object({
    upstream_pollution_keywords: z.string(),
    upstream_pollution_disable_channel: z.boolean(),
    upstream_pollution_message: z.string(),
    upstream_failure_message: z.string(),
  }),
})

type ResponseFilterFormValues = z.infer<typeof responseFilterSchema>

type ResponseFilterOptionValues = {
  'general_setting.upstream_pollution_keywords': string
  'general_setting.upstream_pollution_disable_channel': boolean
  'general_setting.upstream_pollution_message': string
  'general_setting.upstream_failure_message': string
}

type ResponseFilterSectionProps = {
  defaultValues: ResponseFilterOptionValues
}

const toFormValues = (
  values: ResponseFilterOptionValues
): ResponseFilterFormValues => ({
  general_setting: {
    upstream_pollution_keywords:
      values['general_setting.upstream_pollution_keywords'],
    upstream_pollution_disable_channel:
      values['general_setting.upstream_pollution_disable_channel'],
    upstream_pollution_message:
      values['general_setting.upstream_pollution_message'],
    upstream_failure_message:
      values['general_setting.upstream_failure_message'],
  },
})

const toOptionValues = (
  values: ResponseFilterFormValues
): ResponseFilterOptionValues => ({
  'general_setting.upstream_pollution_keywords':
    values.general_setting.upstream_pollution_keywords,
  'general_setting.upstream_pollution_disable_channel':
    values.general_setting.upstream_pollution_disable_channel,
  'general_setting.upstream_pollution_message':
    values.general_setting.upstream_pollution_message,
  'general_setting.upstream_failure_message':
    values.general_setting.upstream_failure_message,
})

const serializeValue = (value: unknown): string => {
  if (typeof value === 'boolean') return String(value)
  return String(value ?? '')
}

export function ResponseFilterSection({
  defaultValues,
}: ResponseFilterSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const formDefaultValues = toFormValues(defaultValues)

  const form = useForm<ResponseFilterFormValues>({
    resolver: zodResolver(responseFilterSchema),
    defaultValues: formDefaultValues,
  })

  useResetForm(form, formDefaultValues)

  const onSubmit = async (values: ResponseFilterFormValues) => {
    const optionValues = toOptionValues(values)
    const updates = Object.entries(optionValues).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof ResponseFilterOptionValues]
    )

    if (updates.length === 0) {
      form.reset(formDefaultValues)
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
                        'If any keyword is found in the upstream response, the system returns your configured message automatically and handles the channel according to the auto-disable switch. No JSON or SSE template is required.'
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
                name='general_setting.upstream_pollution_message'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Pollution response message')}</FormLabel>
                    <FormControl>
                      <Textarea
                        rows={3}
                        placeholder={t(
                          'Plain text returned when an upstream response matches pollution keywords.'
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
                        'Only enter the text you want users to see. The backend automatically wraps it for stream and non-stream responses.'
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
                name='general_setting.upstream_failure_message'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Upstream failure response message')}</FormLabel>
                    <FormControl>
                      <Textarea
                        rows={3}
                        placeholder={t(
                          'Plain text returned when upstream retries are exhausted.'
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
                        'Only enter the text you want users to see. The backend automatically wraps it for Claude, OpenAI, Gemini, stream, and non-stream requests.'
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
