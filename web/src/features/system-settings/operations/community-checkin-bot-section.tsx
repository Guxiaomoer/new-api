import { useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
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
  getCommunityCheckinBotStatus,
  runCommunityCheckinBot,
  type CommunityCheckinBotRunResult,
  type CommunityCheckinBotStatus,
} from './community-checkin-bot-api'

const communityCheckinBotSchema = z.object({
  community_checkin_bot: z.object({
    enabled: z.boolean(),
    bot_user_id: z.string().min(1),
    bot_name: z.string().min(1),
    interval_seconds: z.number().int().min(10),
    min_usd: z.number().int().min(1),
    max_usd: z.number().int().min(1),
    last_message_id: z.string(),
  }),
})

type CommunityCheckinBotFormValues = z.infer<
  typeof communityCheckinBotSchema
>

type CommunityCheckinBotOptionValues = {
  'community_checkin_bot.enabled': boolean
  'community_checkin_bot.bot_user_id': string
  'community_checkin_bot.bot_name': string
  'community_checkin_bot.interval_seconds': number
  'community_checkin_bot.min_usd': number
  'community_checkin_bot.max_usd': number
  'community_checkin_bot.last_message_id': string
}

type CommunityCheckinBotSectionProps = {
  defaultValues: CommunityCheckinBotOptionValues
}

const toFormValues = (
  values: CommunityCheckinBotOptionValues
): CommunityCheckinBotFormValues => ({
  community_checkin_bot: {
    enabled: values['community_checkin_bot.enabled'],
    bot_user_id: values['community_checkin_bot.bot_user_id'],
    bot_name: values['community_checkin_bot.bot_name'],
    interval_seconds: Number(
      values['community_checkin_bot.interval_seconds'] || 30
    ),
    min_usd: Number(values['community_checkin_bot.min_usd'] || 2),
    max_usd: Number(values['community_checkin_bot.max_usd'] || 5),
    last_message_id: values['community_checkin_bot.last_message_id'],
  },
})

const toOptionValues = (
  values: CommunityCheckinBotFormValues
): CommunityCheckinBotOptionValues => ({
  'community_checkin_bot.enabled': values.community_checkin_bot.enabled,
  'community_checkin_bot.bot_user_id': values.community_checkin_bot.bot_user_id,
  'community_checkin_bot.bot_name': values.community_checkin_bot.bot_name,
  'community_checkin_bot.interval_seconds':
    values.community_checkin_bot.interval_seconds,
  'community_checkin_bot.min_usd': values.community_checkin_bot.min_usd,
  'community_checkin_bot.max_usd': values.community_checkin_bot.max_usd,
  'community_checkin_bot.last_message_id':
    values.community_checkin_bot.last_message_id,
})

const serializeValue = (value: unknown): string => {
  if (typeof value === 'boolean') return String(value)
  return String(value ?? '')
}

const formatTime = (timestamp: number) => {
  if (!timestamp) return '从未运行'
  return new Date(timestamp * 1000).toLocaleString()
}

function StatusPanel({
  status,
  result,
}: {
  status: CommunityCheckinBotStatus | null
  result: CommunityCheckinBotRunResult | null
}) {
  if (!status && !result) return null

  return (
    <div className='bg-muted/20 space-y-3 rounded-xl border p-3 text-sm'>
      {status && (
        <>
          <div className='grid gap-2 md:grid-cols-4'>
            <div>
              <div className='text-muted-foreground text-xs'>房间 ID</div>
              <div className='font-medium'>{status.room_id || '未配置'}</div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>授权 Token</div>
              <div className='font-medium'>
                {status.authorization_set ? '已配置' : '未配置'}
              </div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>客户端指纹</div>
              <div className='font-medium'>
                {status.fingerprint_set ? '已配置' : '未配置'}
              </div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>上次检查时间</div>
              <div className='font-medium'>{formatTime(status.last_run_at)}</div>
            </div>
          </div>

          <div className='grid gap-2 md:grid-cols-4'>
            <div>
              <div className='text-muted-foreground text-xs'>上次处理消息数</div>
              <div className='font-medium'>{status.last_processed_count}</div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>上次触发次数</div>
              <div className='font-medium'>{status.last_triggered_count}</div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>上次奖励次数</div>
              <div className='font-medium'>{status.last_rewarded_count}</div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>最后错误</div>
              <div className='font-medium'>{status.last_error || '无'}</div>
            </div>
          </div>
        </>
      )}

      {result && (
        <div className='grid gap-2 border-t pt-3 md:grid-cols-4'>
          <div>
            <div className='text-muted-foreground text-xs'>本次处理消息数</div>
            <div className='font-medium'>{result.processed_count}</div>
          </div>
          <div>
            <div className='text-muted-foreground text-xs'>本次触发次数</div>
            <div className='font-medium'>{result.triggered_count}</div>
          </div>
          <div>
            <div className='text-muted-foreground text-xs'>本次奖励次数</div>
            <div className='font-medium'>{result.rewarded_count}</div>
          </div>
          <div>
            <div className='text-muted-foreground text-xs'>最后处理消息 ID</div>
            <div className='font-medium break-all'>
              {result.last_message_id || '无'}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export function CommunityCheckinBotSection({
  defaultValues,
}: CommunityCheckinBotSectionProps) {
  const updateOption = useUpdateOption()
  const [status, setStatus] = useState<CommunityCheckinBotStatus | null>(null)
  const [lastResult, setLastResult] =
    useState<CommunityCheckinBotRunResult | null>(null)

  const formDefaultValues = toFormValues(defaultValues)
  const form = useForm<CommunityCheckinBotFormValues>({
    resolver: zodResolver(communityCheckinBotSchema.refine(
      (values) =>
        values.community_checkin_bot.max_usd >=
        values.community_checkin_bot.min_usd,
      {
        message: '最大奖励不能小于最小奖励',
        path: ['community_checkin_bot', 'max_usd'],
      }
    )),
    defaultValues: formDefaultValues,
  })

  useResetForm(form, formDefaultValues)

  const statusMutation = useMutation({
    mutationFn: getCommunityCheckinBotStatus,
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || i18next.t('刷新状态失败'))
        return
      }
      setStatus(response.data)
      toast.success(i18next.t('状态已刷新'))
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t('刷新状态失败'))
    },
  })

  const runMutation = useMutation({
    mutationFn: runCommunityCheckinBot,
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || i18next.t('立即检查失败'))
        return
      }
      setLastResult(response.data)
      toast.success(i18next.t('立即检查完成'))
      statusMutation.mutate()
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t('立即检查失败'))
    },
  })

  const onSubmit = async (values: CommunityCheckinBotFormValues) => {
    const optionValues = toOptionValues(values)
    const updates = Object.entries(optionValues).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof CommunityCheckinBotOptionValues]
    )

    if (updates.length === 0) {
      form.reset(formDefaultValues)
      toast.info(i18next.t('没有需要保存的修改'))
      return
    }

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: serializeValue(value) })
    }
  }

  const isDirty = form.formState.isDirty
  const isSubmitting = form.formState.isSubmitting
  const handleSubmit = form.handleSubmit(onSubmit)
  const isBusy =
    updateOption.isPending ||
    isSubmitting ||
    statusMutation.isPending ||
    runMutation.isPending

  return (
    <SettingsSection title='社区签到机器人维护'>
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
                name='community_checkin_bot.enabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>启用社区签到机器人</FormLabel>
                      <FormDescription>
                        启用后，主节点会轮询社区群消息，并处理 @机器人 的“签到”请求。
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

            <FormField
              control={form.control}
              name='community_checkin_bot.bot_user_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>机器人用户 ID</FormLabel>
                  <FormControl>
                    <Input {...field} disabled={isBusy} />
                  </FormControl>
                  <FormDescription>用于匹配 mentionedUserIds。</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='community_checkin_bot.bot_name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>机器人名称</FormLabel>
                  <FormControl>
                    <Input {...field} disabled={isBusy} />
                  </FormControl>
                  <FormDescription>用于清理消息里的 @名称。</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='community_checkin_bot.interval_seconds'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>轮询间隔（秒）</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={10}
                      value={String(field.value ?? '')}
                      onChange={(event) =>
                        field.onChange(Number(event.target.value))
                      }
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                      disabled={isBusy}
                    />
                  </FormControl>
                  <FormDescription>最小 10 秒，默认 30 秒。</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='community_checkin_bot.min_usd'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>最小奖励（美元）</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      value={String(field.value ?? '')}
                      onChange={(event) =>
                        field.onChange(Number(event.target.value))
                      }
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                      disabled={isBusy}
                    />
                  </FormControl>
                  <FormDescription>默认 2 美元。</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='community_checkin_bot.max_usd'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>最大奖励（美元）</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      value={String(field.value ?? '')}
                      onChange={(event) =>
                        field.onChange(Number(event.target.value))
                      }
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                      disabled={isBusy}
                    />
                  </FormControl>
                  <FormDescription>默认 5 美元。</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='community_checkin_bot.last_message_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>最后处理消息 ID</FormLabel>
                    <FormControl>
                      <Input {...field} disabled={isBusy} />
                    </FormControl>
                    <FormDescription>
                      留空会从最近消息开始处理；通常仅排查重复处理时手动调整。
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
              onClick={() => statusMutation.mutate()}
              disabled={isBusy}
            >
              {statusMutation.isPending ? '刷新中...' : '刷新状态'}
            </Button>
            <Button
              type='button'
              onClick={() => runMutation.mutate()}
              disabled={isBusy}
            >
              {runMutation.isPending ? '检查中...' : '立即检查'}
            </Button>
          </div>

          <StatusPanel status={status} result={lastResult} />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
