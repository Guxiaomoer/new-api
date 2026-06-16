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
import { useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
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
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
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
import {
  previewCommunitySync,
  runCommunitySync,
  type CommunitySyncResult,
} from './community-sync-api'

const communitySyncSchema = z.object({
  community_sync: z.object({
    enabled: z.boolean(),
    endpoint: z.string().url(),
    room_id: z.string().min(1),
    authorization: z.string(),
    fingerprint: z.string(),
    interval_minutes: z.coerce.number().int().min(1).max(1440),
    protected_users: z.string(),
  }),
})

type CommunitySyncFormInput = z.input<typeof communitySyncSchema>
type CommunitySyncFormValues = z.output<typeof communitySyncSchema>

type CommunitySyncOptionValues = {
  'community_sync.enabled': boolean
  'community_sync.endpoint': string
  'community_sync.room_id': string
  'community_sync.authorization': string
  'community_sync.fingerprint': string
  'community_sync.interval_minutes': number
  'community_sync.protected_users': string
}

type CommunitySyncSectionProps = {
  defaultValues: CommunitySyncOptionValues
}

const toFormValues = (
  values: CommunitySyncOptionValues
): CommunitySyncFormInput => ({
  community_sync: {
    enabled: values['community_sync.enabled'],
    endpoint: values['community_sync.endpoint'],
    room_id: values['community_sync.room_id'],
    authorization: values['community_sync.authorization'],
    fingerprint: values['community_sync.fingerprint'],
    interval_minutes: Number(values['community_sync.interval_minutes'] || 5),
    protected_users: values['community_sync.protected_users'],
  },
})

const toOptionValues = (
  values: CommunitySyncFormValues
): CommunitySyncOptionValues => ({
  'community_sync.enabled': values.community_sync.enabled,
  'community_sync.endpoint': values.community_sync.endpoint,
  'community_sync.room_id': values.community_sync.room_id,
  'community_sync.authorization': values.community_sync.authorization,
  'community_sync.fingerprint': values.community_sync.fingerprint,
  'community_sync.interval_minutes': values.community_sync.interval_minutes,
  'community_sync.protected_users': values.community_sync.protected_users,
})

const serializeValue = (value: unknown): string => {
  if (typeof value === 'boolean') return String(value)
  return String(value ?? '')
}

function ResultPanel({ result }: { result: CommunitySyncResult | null }) {
  const { t } = useTranslation()

  if (!result) return null

  return (
    <div className='bg-muted/20 space-y-3 rounded-xl border p-3 text-sm'>
      <div className='grid gap-2 md:grid-cols-5'>
        <div>
          <div className='text-muted-foreground text-xs'>{t('Community members')}</div>
          <div className='font-medium'>{result.member_count}</div>
        </div>
        <div>
          <div className='text-muted-foreground text-xs'>{t('Local users')}</div>
          <div className='font-medium'>{result.local_user_count}</div>
        </div>
        <div>
          <div className='text-muted-foreground text-xs'>{t('Will restrict')}</div>
          <div className='font-medium'>{result.restricted_count}</div>
        </div>
        <div>
          <div className='text-muted-foreground text-xs'>{t('Will unrestrict')}</div>
          <div className='font-medium'>{result.unrestricted_count}</div>
        </div>
        <div>
          <div className='text-muted-foreground text-xs'>{t('Protected skipped')}</div>
          <div className='font-medium'>{result.protected_skipped}</div>
        </div>
      </div>

      <div className='grid gap-3 md:grid-cols-2'>
        <div>
          <div className='mb-1 font-medium'>{t('Users to restrict')}</div>
          <pre className='bg-background max-h-40 overflow-auto rounded-md border p-2 text-xs'>
            {(result.restrict_users || []).join('\n') || t('None')}
          </pre>
        </div>
        <div>
          <div className='mb-1 font-medium'>{t('Users to unrestrict')}</div>
          <pre className='bg-background max-h-40 overflow-auto rounded-md border p-2 text-xs'>
            {(result.unrestrict_users || []).join('\n') || t('None')}
          </pre>
        </div>
      </div>
    </div>
  )
}

export function CommunitySyncSection({
  defaultValues,
}: CommunitySyncSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [lastResult, setLastResult] = useState<CommunitySyncResult | null>(null)

  const formDefaultValues = toFormValues(defaultValues)
  const form = useForm<CommunitySyncFormInput, unknown, CommunitySyncFormValues>({
    resolver: zodResolver(communitySyncSchema),
    defaultValues: formDefaultValues,
  })

  useResetForm(form, formDefaultValues)

  const previewMutation = useMutation({
    mutationFn: previewCommunitySync,
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || i18next.t('Preview failed'))
        return
      }
      setLastResult(response.data)
      toast.success(i18next.t('Preview completed'))
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t('Preview failed'))
    },
  })

  const syncMutation = useMutation({
    mutationFn: runCommunitySync,
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || i18next.t('Sync failed'))
        return
      }
      setLastResult(response.data)
      toast.success(i18next.t('Community member sync completed'))
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t('Sync failed'))
    },
  })

  const onSubmit = async (values: CommunitySyncFormValues) => {
    const optionValues = toOptionValues(values)
    const updates = Object.entries(optionValues).filter(([key, value]) => {
      if (key === 'community_sync.authorization' && String(value).trim() === '') {
        return false
      }
      return value !== defaultValues[key as keyof CommunitySyncOptionValues]
    })

    if (updates.length === 0) {
      form.reset(formDefaultValues)
      toast.info(i18next.t('No changes to save'))
      return
    }

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: serializeValue(value) })
    }
    form.setValue('community_sync.authorization', '')
  }

  const isDirty = form.formState.isDirty
  const isSubmitting = form.formState.isSubmitting
  const handleSubmit = form.handleSubmit(onSubmit)
  const isBusy =
    updateOption.isPending ||
    isSubmitting ||
    previewMutation.isPending ||
    syncMutation.isPending

  return (
    <SettingsSection title={t('Community Member Sync')}>
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
                name='community_sync.enabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Enable community member sync')}</FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, the server checks the configured community room every interval and only changes API restriction after a full successful member fetch.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={isBusy}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='community_sync.endpoint'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Community members API endpoint')}</FormLabel>
                    <FormControl>
                      <Input {...field} disabled={isBusy} />
                    </FormControl>
                    <FormDescription>
                      {t('Default endpoint: https://dc.hhhl.cc/api/chat/rooms/members')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <FormField
              control={form.control}
              name='community_sync.room_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Room ID')}</FormLabel>
                  <FormControl>
                    <Input {...field} disabled={isBusy} />
                  </FormControl>
                  <FormDescription>{t('Default room ID: ani5zrxyl7')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='community_sync.interval_minutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Sync interval minutes')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      max={1440}
                      value={String(field.value ?? '')}
                      onChange={(event) => field.onChange(event.target.value)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                      disabled={isBusy}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Use 5 for near real-time membership updates.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='community_sync.authorization'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Authorization token')}</FormLabel>
                    <FormControl>
                      <Input
                        type='password'
                        autoComplete='new-password'
                        placeholder={t('Leave blank to keep the existing token')}
                        {...field}
                        disabled={isBusy}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Stored server-side only. The existing token is never returned to the browser.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='community_sync.fingerprint'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Client fingerprint')}</FormLabel>
                    <FormControl>
                      <Input {...field} disabled={isBusy} />
                    </FormControl>
                    <FormDescription>
                      {t('Optional x-client-fingerprint header required by the community API.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='community_sync.protected_users'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Protected whitelist')}</FormLabel>
                    <FormControl>
                      <Textarea rows={5} {...field} disabled={isBusy} />
                    </FormControl>
                    <FormDescription>
                      {t('One username, display name, or email per line. These users are never changed by sync.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>
          </SettingsFormGrid>

          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              variant='outline'
              onClick={() => previewMutation.mutate()}
              disabled={isBusy}
            >
              {previewMutation.isPending ? t('Previewing...') : t('Preview sync')}
            </Button>
            <Button
              type='button'
              onClick={() => syncMutation.mutate()}
              disabled={isBusy}
            >
              {syncMutation.isPending ? t('Syncing...') : t('Sync now')}
            </Button>
          </div>

          <ResultPanel result={lastResult} />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
