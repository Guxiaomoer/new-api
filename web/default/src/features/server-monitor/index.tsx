import type { ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  Activity,
  Clock,
  Cpu,
  Database,
  HardDrive,
  MemoryStick,
  RefreshCcw,
  Server,
  Users,
  Zap,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { getServerMonitorOverview } from './api'
import type {
  ServerMonitorCapacity,
  ServerMonitorLoad,
  ServerMonitorOverview,
  ServerMonitorUsage,
} from './types'

const queryKey = ['server-monitor-overview']

export function ServerMonitor() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const overviewQuery = useQuery({
    queryKey,
    queryFn: async () => {
      const result = await getServerMonitorOverview()
      if (!result.success) throw new Error(result.message)
      return result.data
    },
    refetchInterval: 15_000,
  })

  const overview = overviewQuery.data
  const isBusy = overviewQuery.isFetching

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey })
    toast.success(t('Refreshing server monitor'))
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Server Monitor')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='outline' onClick={refresh} disabled={isBusy}>
          <RefreshCcw className={isBusy ? 'animate-spin' : ''} />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          {overviewQuery.error ? (
            <Alert variant='destructive'>
              <AlertTriangle />
              <AlertTitle>{t('Failed to load server monitor')}</AlertTitle>
              <AlertDescription>
                {overviewQuery.error instanceof Error
                  ? overviewQuery.error.message
                  : t('Please try again later')}
              </AlertDescription>
            </Alert>
          ) : null}

          {overview ? (
            <ServerMonitorContent overview={overview} />
          ) : (
            <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
              {Array.from({ length: 4 }).map((_, index) => (
                <Card key={index} className='min-h-36 animate-pulse' />
              ))}
            </div>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function ServerMonitorContent({ overview }: { overview: ServerMonitorOverview }) {
  const { t } = useTranslation()
  const collectedAt = new Date(overview.collected_at * 1000).toLocaleString()

  return (
    <div className='space-y-4'>
      <div className='flex flex-wrap items-center gap-2 text-sm text-muted-foreground'>
        <CapacityBadge capacity={overview.capacity} />
        <span>
          {t('Last collected')}: {collectedAt}
        </span>
        {overview.partial ? (
          <Badge variant='secondary'>{t('Partial data')}</Badge>
        ) : null}
      </div>

      {overview.warnings.length > 0 ? (
        <Alert>
          <AlertTriangle />
          <AlertTitle>{t('Warnings and suggestions')}</AlertTitle>
          <AlertDescription>
            <ul className='list-disc space-y-1 pl-4'>
              {overview.warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      ) : null}

      <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
        <MetricCard
          title={t('CPU')}
          icon={<Cpu className='size-4' />}
          value={`${formatPercent(overview.host.cpu_usage_percent)}%`}
          description={`${overview.host.cpu_cores} ${t('cores')} · load ${formatLoad(overview.host.load_average)}`}
          progress={overview.host.cpu_usage_percent}
        />
        <UsageCard
          title={t('Memory')}
          icon={<MemoryStick className='size-4' />}
          usage={overview.host.memory}
        />
        <UsageCard
          title={t('Swap')}
          icon={<Zap className='size-4' />}
          usage={overview.host.swap}
        />
        <UsageCard
          title={t('Root Disk')}
          icon={<HardDrive className='size-4' />}
          usage={overview.host.root_disk}
        />
      </div>

      <div className='grid gap-4 lg:grid-cols-3'>
        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2 text-base'>
              <Server className='size-4' />
              {t('Server Status')}
            </CardTitle>
          </CardHeader>
          <CardContent className='grid gap-3 text-sm'>
            <InfoRow
              label={t('Uptime')}
              value={formatDuration(overview.host.uptime_seconds)}
            />
            <InfoRow
              label={t('CPU Cores')}
              value={overview.host.cpu_cores.toString()}
            />
            <InfoRow
              label={t('Load Average')}
              value={formatLoad(overview.host.load_average)}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2 text-base'>
              <Activity className='size-4' />
              {t('New API Process')}
            </CardTitle>
          </CardHeader>
          <CardContent className='grid gap-3 text-sm'>
            <InfoRow label={t('Go Version')} value={overview.app.go_version} />
            <InfoRow
              label={t('Goroutines')}
              value={formatNumber(overview.app.goroutines)}
            />
            <InfoRow
              label={t('Heap Alloc')}
              value={formatBytes(overview.app.heap_alloc)}
            />
            <InfoRow
              label={t('Heap Sys')}
              value={formatBytes(overview.app.heap_sys)}
            />
            <InfoRow
              label={t('Runtime Sys')}
              value={formatBytes(overview.app.sys)}
            />
            <InfoRow
              label={t('GC Count')}
              value={formatNumber(overview.app.num_gc)}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2 text-base'>
              <Database className='size-4' />
              {t('User Capacity')}
            </CardTitle>
          </CardHeader>
          <CardContent className='grid gap-3 text-sm'>
            <InfoRow
              label={t('Total Users')}
              value={formatNumber(overview.database.total_users)}
            />
            <InfoRow
              label={t('Enabled Users')}
              value={formatNumber(overview.database.enabled_users)}
            />
            <InfoRow
              label={t('OAuth Bindings')}
              value={formatNumber(overview.database.oauth_bindings)}
            />
            <InfoRow
              label={t('Total Tokens')}
              value={formatNumber(overview.database.total_tokens)}
            />
            <InfoRow
              label={t('Enabled Tokens')}
              value={formatNumber(overview.database.enabled_tokens)}
            />
            <InfoRow
              label={t('New Users 24h')}
              value={formatNumber(overview.database.recent_users_24h)}
            />
            <InfoRow
              label={t('Logins 24h')}
              value={formatNumber(overview.database.recent_logins_24h)}
            />
            <InfoRow
              label={t('Logins 7d')}
              value={formatNumber(overview.database.recent_logins_7d)}
            />
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-base'>
            <Users className='size-4' />
            {t('Capacity Recommendation')}
          </CardTitle>
        </CardHeader>
        <CardContent className='space-y-3 text-sm'>
          <InfoRow
            label={t('Concurrent Requests')}
            value={overview.capacity.conservative_concurrent_range}
          />
          <InfoRow
            label={t('Registered Users')}
            value={overview.capacity.registered_users_suggestion}
          />
          <div className='rounded-lg border bg-muted/40 p-3'>
            <div className='mb-2 flex items-center gap-2 font-medium'>
              <Clock className='size-4' />
              {t('Operational hints')}
            </div>
            <ul className='list-disc space-y-1 pl-4 text-muted-foreground'>
              {overview.capacity.hints.map((hint) => (
                <li key={hint}>{hint}</li>
              ))}
            </ul>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function MetricCard({
  title,
  icon,
  value,
  description,
  progress,
}: {
  title: string
  icon: ReactNode
  value: string
  description: string
  progress: number
}) {
  return (
    <Card>
      <CardHeader className='pb-2'>
        <CardTitle className='flex items-center gap-2 text-sm font-medium text-muted-foreground'>
          {icon}
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className='space-y-3'>
        <div className='text-2xl font-bold'>{value}</div>
        <Progress value={clampPercent(progress)} />
        <p className='text-xs text-muted-foreground'>{description}</p>
      </CardContent>
    </Card>
  )
}

function UsageCard({
  title,
  icon,
  usage,
}: {
  title: string
  icon: ReactNode
  usage: ServerMonitorUsage
}) {
  return (
    <MetricCard
      title={title}
      icon={icon}
      value={`${formatPercent(usage.used_percent)}%`}
      description={`${formatBytes(usage.used)} / ${formatBytes(usage.total)} · available ${formatBytes(usage.available)}`}
      progress={usage.used_percent}
    />
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className='flex items-start justify-between gap-3'>
      <span className='text-muted-foreground'>{label}</span>
      <span className='text-right font-medium'>{value}</span>
    </div>
  )
}

function CapacityBadge({ capacity }: { capacity: ServerMonitorCapacity }) {
  const variant = capacity.level === 'critical' ? 'destructive' : 'secondary'
  const label =
    capacity.level === 'ok'
      ? '正常'
      : capacity.level === 'warning'
        ? '注意'
        : '危险'
  return <Badge variant={variant}>{label}</Badge>
}

function formatBytes(bytes: number, decimals = 2): string {
  if (!bytes || isNaN(bytes)) return '0 Bytes'
  if (bytes === 0) return '0 Bytes'
  if (bytes < 0) return '-' + formatBytes(-bytes, decimals)
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(k))
  if (i < 0 || i >= sizes.length) return `${bytes} Bytes`
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(decimals))} ${sizes[i]}`
}

function formatDuration(seconds: number): string {
  if (!seconds || seconds < 0) return '0 分钟'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days} 天 ${hours} 小时`
  if (hours > 0) return `${hours} 小时 ${minutes} 分钟`
  return `${minutes} 分钟`
}

function formatLoad(load: ServerMonitorLoad): string {
  return `${load.one_minute.toFixed(2)} / ${load.five_minutes.toFixed(2)} / ${load.fifteen_minutes.toFixed(2)}`
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value || 0)
}

function formatPercent(value: number): string {
  return clampPercent(value).toFixed(1)
}

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, value))
}
