import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  Play,
  RefreshCcw,
  Save,
  ShieldCheck,
  Square,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  detectCommunityMonitor,
  getCommunityMonitorResults,
  getCommunityMonitorStatus,
  saveCommunityMonitorConfig,
  scanCommunityMonitor,
  startCommunityMonitorCollector,
  stopCommunityMonitorCollector,
} from './api'
import type { CommunityMonitorConfig, CommunityMonitorStatus } from './types'

const DEFAULT_CONFIG: CommunityMonitorConfig = {
  source_url: '',
  room_id: '',
  user_id: '',
  room_url: '',
  start_time: '',
  end_time: '',
  query: 'sk-',
  extract_regex: 'sk-[A-Za-z0-9_-]{8,}',
  scan_limit: 100,
  page_size: 30,
  collector_interval_minutes: 10,
  access_token: '',
  headers: {},
  detection_base_url: '',
}

const queryKey = ['community-monitor']
const resultsQueryKey = ['community-monitor-results']

export function CommunityMonitor({ embedded = false }: { embedded?: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [config, setConfig] = useState<CommunityMonitorConfig>(DEFAULT_CONFIG)
  const [headersText, setHeadersText] = useState('{}')

  const statusQuery = useQuery({
    queryKey,
    queryFn: async () => {
      const result = await getCommunityMonitorStatus()
      if (!result.success) throw new Error(result.message)
      return result.data
    },
  })

  const resultsQuery = useQuery({
    queryKey: resultsQueryKey,
    queryFn: async () => {
      const result = await getCommunityMonitorResults()
      if (!result.success) throw new Error(result.message)
      return result.data || []
    },
  })

  useEffect(() => {
    if (statusQuery.data?.config) {
      setConfig({ ...DEFAULT_CONFIG, ...statusQuery.data.config, access_token: '' })
      setHeadersText(JSON.stringify(statusQuery.data.config.headers || {}, null, 2))
    }
  }, [statusQuery.data?.config])

  const status = statusQuery.data
  const state = status?.state
  const progress = state?.progress
  const results = resultsQuery.data || status?.state.results || []
  const isBusy = statusQuery.isFetching || resultsQuery.isFetching

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey })
    queryClient.invalidateQueries({ queryKey: resultsQueryKey })
  }

  const saveMutation = useMutation({
    mutationFn: async () => {
      let headers: Record<string, string> = {}
      try {
        headers = headersText.trim() ? JSON.parse(headersText) : {}
      } catch {
        throw new Error(t('Headers must be valid JSON'))
      }
      const result = await saveCommunityMonitorConfig({ ...config, headers })
      if (!result.success) throw new Error(result.message)
      return result.data
    },
    onSuccess: () => {
      toast.success(t('Saved'))
      invalidate()
    },
    onError: (error) => toast.error(error.message),
  })

  const actionMutation = useMutation({
    mutationFn: async (action: 'scan' | 'detect' | 'start' | 'stop') => {
      const calls = {
        scan: scanCommunityMonitor,
        detect: detectCommunityMonitor,
        start: startCommunityMonitorCollector,
        stop: stopCommunityMonitorCollector,
      }
      const result = await calls[action]()
      if (!result.success) throw new Error(result.message)
      return result.data
    },
    onSuccess: () => invalidate(),
    onError: (error) => toast.error(error.message),
  })

  const rules = status?.rules || []
  const validResults = useMemo(
    () => results.filter((result) => result.status === 'reachable'),
    [results]
  )

  const content = (
    <div className='space-y-4'>
      <StatusHeader status={status} />

      <div className='grid gap-4 xl:grid-cols-[390px_1fr]'>
        <div className='space-y-4'>
              <Card>
                <CardHeader>
                  <CardTitle>{t('Search Conditions')}</CardTitle>
                </CardHeader>
                <CardContent className='space-y-3'>
                  <Field label={t('Source URL')}>
                    <Input
                      value={config.source_url}
                      onChange={(event) =>
                        setConfig({ ...config, source_url: event.target.value })
                      }
                    />
                  </Field>
                  <div className='grid grid-cols-2 gap-3'>
                    <Field label={t('Room ID')}>
                      <Input
                        value={config.room_id}
                        onChange={(event) =>
                          setConfig({ ...config, room_id: event.target.value })
                        }
                      />
                    </Field>
                    <Field label={t('User ID')}>
                      <Input
                        value={config.user_id}
                        onChange={(event) =>
                          setConfig({ ...config, user_id: event.target.value })
                        }
                      />
                    </Field>
                  </div>
                  <Field label={t('Room URL')}>
                    <Input
                      value={config.room_url}
                      onChange={(event) =>
                        setConfig({ ...config, room_url: event.target.value })
                      }
                    />
                  </Field>
                  <div className='grid grid-cols-2 gap-3'>
                    <Field label={t('Start Time')}>
                      <Input
                        type='datetime-local'
                        value={config.start_time}
                        onChange={(event) =>
                          setConfig({ ...config, start_time: event.target.value })
                        }
                      />
                    </Field>
                    <Field label={t('End Time')}>
                      <Input
                        type='datetime-local'
                        value={config.end_time}
                        onChange={(event) =>
                          setConfig({ ...config, end_time: event.target.value })
                        }
                      />
                    </Field>
                  </div>
                  <Field label={t('Access Token')}>
                    <Input
                      type='password'
                      placeholder={
                        config.access_token_configured
                          ? t('Configured, leave empty to keep unchanged')
                          : ''
                      }
                      value={config.access_token || ''}
                      onChange={(event) =>
                        setConfig({ ...config, access_token: event.target.value })
                      }
                    />
                  </Field>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t('Match Rules')}</CardTitle>
                </CardHeader>
                <CardContent className='space-y-3'>
                  <Field label={t('Query')}>
                    <Input
                      value={config.query}
                      onChange={(event) =>
                        setConfig({ ...config, query: event.target.value })
                      }
                    />
                  </Field>
                  <Field label={t('Extract Regex')}>
                    <Input
                      value={config.extract_regex}
                      onChange={(event) =>
                        setConfig({ ...config, extract_regex: event.target.value })
                      }
                    />
                  </Field>
                  <div className='flex flex-wrap gap-2'>
                    {rules.map((rule) => (
                      <Button
                        key={rule.name}
                        size='sm'
                        variant='outline'
                        onClick={() =>
                          setConfig({
                            ...config,
                            query: rule.query,
                            extract_regex: rule.regex,
                          })
                        }
                      >
                        {t(rule.name)}
                      </Button>
                    ))}
                  </div>
                  <div className='grid grid-cols-2 gap-3'>
                    <Field label={t('Scan Limit')}>
                      <Input
                        type='number'
                        value={config.scan_limit}
                        onChange={(event) =>
                          setConfig({
                            ...config,
                            scan_limit: Number(event.target.value),
                          })
                        }
                      />
                    </Field>
                    <Field label={t('Page Size')}>
                      <Input
                        type='number'
                        value={config.page_size}
                        onChange={(event) =>
                          setConfig({
                            ...config,
                            page_size: Number(event.target.value),
                          })
                        }
                      />
                    </Field>
                  </div>
                  <Field label={t('Detection Base URL')}>
                    <Input
                      value={config.detection_base_url}
                      onChange={(event) =>
                        setConfig({
                          ...config,
                          detection_base_url: event.target.value,
                        })
                      }
                    />
                  </Field>
                  <Field label={t('Request Headers JSON')}>
                    <Textarea
                      value={headersText}
                      onChange={(event) => setHeadersText(event.target.value)}
                    />
                  </Field>
                </CardContent>
              </Card>
            </div>

            <div className='space-y-4'>
              <Card>
                <CardHeader>
                  <CardTitle>{t('Scan Progress')}</CardTitle>
                </CardHeader>
                <CardContent className='space-y-4'>
                  <div className='grid gap-3 sm:grid-cols-5'>
                    <Stat label={t('Checked')} value={progress?.checked || 0} />
                    <Stat label={t('Read')} value={progress?.read || 0} />
                    <Stat label={t('Pages')} value={progress?.pages || 0} />
                    <Stat label={t('Hits')} value={progress?.hits || 0} />
                    <Stat label={t('Duplicates')} value={progress?.duplicates || 0} />
                  </div>
                  <Progress value={progress?.percent || 0} />
                  <div className='text-muted-foreground text-xs'>
                    {progress?.percent || 0}%
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t('Collector')}</CardTitle>
                </CardHeader>
                <CardContent className='space-y-4'>
                  <div className='flex items-center gap-2'>
                    <Badge>{status?.running ? t('Running') : t('Stopped')}</Badge>
                    <span className='text-muted-foreground text-sm'>
                      {state?.last_error || t('No Error')}
                    </span>
                  </div>
                  <div className='grid gap-3 sm:grid-cols-4'>
                    <Stat label={t('Messages')} value={state?.message_count || 0} />
                    <Stat label={t('Hits')} value={progress?.hits || 0} />
                    <Stat
                      label={t('Candidates')}
                      value={state?.candidate_count || 0}
                    />
                    <Stat
                      label={t('Detected')}
                      value={state?.detected_count || 0}
                    />
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='mb-2 flex items-center gap-2 text-sm font-medium'>
                      {t('Valid Secrets')}
                      <Badge variant='secondary'>
                        {state?.valid_count || 0}/{state?.candidate_count || 0}
                      </Badge>
                    </div>
                    <div className='flex flex-wrap gap-2'>
                      {validResults.length ? (
                        validResults.map((result) => (
                          <Badge key={result.fingerprint} variant='secondary'>
                            {result.masked_value}
                          </Badge>
                        ))
                      ) : (
                        <span className='text-muted-foreground text-sm'>
                          {t('No valid secrets')}
                        </span>
                      )}
                    </div>
                  </div>
                  <div className='text-muted-foreground grid gap-1 text-sm sm:grid-cols-2'>
                    <span>
                      {t('Scan Window')}: {progress?.scan_started_at || '-'} -{' '}
                      {progress?.scan_ended_at || '-'}
                    </span>
                    <span>
                      {t('Next Run')}: {state?.next_run_at || '-'}
                    </span>
                    <span>
                      {t('State File')}: {status?.config.state_path || '-'}
                    </span>
                    <span>
                      {t('Failure Cache')}: {state?.failure_cache || 0}
                    </span>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t('Results')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Result')}</TableHead>
                        <TableHead>{t('Secret')}</TableHead>
                        <TableHead>{t('Reason')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {results.length ? (
                        results.map((result) => (
                          <TableRow key={result.fingerprint}>
                            <TableCell>{result.status}</TableCell>
                            <TableCell className='font-mono'>
                              {result.masked_value}
                            </TableCell>
                            <TableCell>{result.reason}</TableCell>
                          </TableRow>
                        ))
                      ) : (
                        <TableRow>
                          <TableCell colSpan={3} className='text-muted-foreground'>
                            {t('No community monitor data')}
                          </TableCell>
                        </TableRow>
                      )}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            </div>
          </div>
        </div>
  )

  if (embedded) {
    return content
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Community Monitor')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='outline' onClick={() => invalidate()} disabled={isBusy}>
          <RefreshCcw /> {t('Refresh')}
        </Button>
        <Button onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending}>
          <Save /> {t('Save')}
        </Button>
        <Button
          variant='outline'
          onClick={() => actionMutation.mutate('scan')}
          disabled={actionMutation.isPending}
        >
          <Play /> {t('Scan')}
        </Button>
        <Button
          variant='outline'
          onClick={() => actionMutation.mutate('detect')}
          disabled={actionMutation.isPending}
        >
          <ShieldCheck /> {t('Detect')}
        </Button>
        <Button
          variant='destructive'
          onClick={() => actionMutation.mutate(status?.running ? 'stop' : 'start')}
          disabled={actionMutation.isPending}
        >
          {status?.running ? <Square /> : <Play />}
          {status?.running ? t('Stop Collector') : t('Start Collector')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function StatusHeader({ status }: { status?: CommunityMonitorStatus }) {
  const { t } = useTranslation()
  return (
    <div className='flex flex-wrap items-center gap-2 text-sm'>
      <Badge variant={status?.config.access_token_configured ? 'default' : 'outline'}>
        {status?.config.access_token_configured
          ? t('Access token configured')
          : t('Access token not configured')}
      </Badge>
      <Badge variant={status?.running ? 'default' : 'secondary'}>
        {status?.running ? t('Collector running') : t('Collector stopped')}
      </Badge>
      <span className='text-muted-foreground'>
        {status?.config.config_path || 'data/community-monitor/config.json'}
      </span>
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className='space-y-1 text-sm font-medium'>
      <span>{label}</span>
      {children}
    </label>
  )
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className='rounded-lg border p-3'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='mt-2 text-2xl font-semibold'>{value}</div>
    </div>
  )
}
