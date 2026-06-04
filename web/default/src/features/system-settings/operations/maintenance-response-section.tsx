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

const maintenanceResponseSchema = z.object({
  'general_setting.global_maintenance_enabled': z.boolean(),
  'general_setting.global_maintenance_message': z.string(),
  'general_setting.global_maintenance_json_template': z.string(),
  'general_setting.global_maintenance_stream_template': z.string(),
})

type MaintenanceResponseFormValues = z.infer<typeof maintenanceResponseSchema>

type MaintenanceResponseSectionProps = {
  defaultValues: MaintenanceResponseFormValues
}

const serializeValue = (value: unknown): string => {
  if (typeof value === 'boolean') return String(value)
  return String(value ?? '')
}

export function MaintenanceResponseSection({
  defaultValues,
}: MaintenanceResponseSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<MaintenanceResponseFormValues>({
    resolver: zodResolver(maintenanceResponseSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (values: MaintenanceResponseFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof MaintenanceResponseFormValues]
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
                        'This configured message is the default source for global maintenance, channel maintenance fallback, and failed advanced-template fallback. The backend automatically wraps it for Claude, OpenAI, Gemini, stream, and non-stream requests.'
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
                name='general_setting.global_maintenance_json_template'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Global maintenance non-stream template')}
                    </FormLabel>
                    <FormControl>
                      <Textarea
                        rows={8}
                        placeholder={t(
                          'Optional advanced JSON template. Leave empty to use the default maintenance message above. Example: {"error":{"message":"Service is under maintenance","type":"maintenance","code":"maintenance"}}'
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
                        'Used only when global maintenance is enabled and the default maintenance message is empty. Must render valid JSON. Variables: {{.Model}} {{.RequestId}} {{.Created}} {{.Timestamp}}. Use {{json .Model}} inside JSON strings.'
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
                name='general_setting.global_maintenance_stream_template'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Global maintenance stream template')}
                    </FormLabel>
                    <FormControl>
                      <Textarea
                        rows={8}
                        placeholder={t(
                          'Optional advanced SSE template. Leave empty to use the default maintenance message above. Example:\ndata: {"choices":[{"delta":{"content":"Service is under maintenance"}}]}\n\ndata: [DONE]\n\n'
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
                        'Used only when global maintenance is enabled and the default maintenance message is empty. Include complete SSE frames yourself. If you put Model into SSE JSON, use {{json .Model}} or {{.ModelJSON}} instead of raw {{.Model}}.'
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
