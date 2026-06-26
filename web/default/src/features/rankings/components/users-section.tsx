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
import { useMemo } from 'react'
import { VChart } from '@visactor/react-vchart'
import { Trophy, User } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useChartTheme } from '@/lib/use-chart-theme'
import { VCHART_OPTION } from '@/lib/vchart'
import { formatTokens } from '../lib/format'
import type { RankingPeriod, UserHistorySeries, UserRanking } from '../types'
import { GrowthText } from './growth-text'

const PERIOD_DESCRIPTIONS: Record<RankingPeriod, string> = {
  today: 'Hourly token usage by user across the last 24 hours',
  week: 'Weekly token usage by user across the past few weeks',
  month: 'Daily token usage by user across the past month',
  year: 'Weekly token usage by user across the past year',
}

type UsersSectionProps = {
  history: UserHistorySeries
  rows: UserRanking[]
  period: RankingPeriod
}

export function UsersSection(props: UsersSectionProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()
  const chartTextColor =
    resolvedTheme === 'dark'
      ? 'rgba(255, 255, 255, 0.68)'
      : 'rgba(15, 23, 42, 0.58)'
  const chartGridColor =
    resolvedTheme === 'dark'
      ? 'rgba(255, 255, 255, 0.12)'
      : 'rgba(15, 23, 42, 0.12)'

  const orderedPoints = useMemo(() => {
    const order = new Map(
      props.history.users.map((u, idx) => [u.username, idx] as const)
    )
    return [...props.history.points].sort((a, b) => {
      const tsCmp = a.ts.localeCompare(b.ts)
      if (tsCmp !== 0) return tsCmp
      return (order.get(a.username) ?? 999) - (order.get(b.username) ?? 999)
    })
  }, [props.history])

  const totalTokens = useMemo(
    () => props.rows.reduce((s, r) => s + r.total_tokens, 0),
    [props.rows]
  )

  const spec = useMemo(() => {
    if (orderedPoints.length === 0) return null
    return {
      type: 'bar' as const,
      data: [{ id: 'users-history', values: orderedPoints }],
      xField: 'label',
      yField: 'tokens',
      seriesField: 'username',
      stack: true,
      legends: { visible: false },
      axes: [
        {
          orient: 'bottom',
          label: {
            style: { fill: chartTextColor, fontSize: 10 },
            autoHide: true,
            autoLimit: true,
          },
          tick: { visible: false },
        },
        {
          orient: 'left',
          label: {
            formatMethod: (val: number | string) => formatTokens(Number(val)),
            style: { fill: chartTextColor, fontSize: 10 },
          },
          grid: {
            visible: true,
            style: { lineDash: [3, 3], stroke: chartGridColor },
          },
        },
      ],
      tooltip: {
        dimension: {
          title: {
            value: (datum: Record<string, unknown>) =>
              String(datum?.label ?? ''),
          },
          content: [
            {
              key: (datum: Record<string, unknown>) =>
                String(datum?.username ?? ''),
              value: (datum: Record<string, unknown>) =>
                Number(datum?.tokens) || 0,
            },
          ],
          updateContent: (
            array: Array<{ key: string; value: string | number }>
          ) => {
            array.sort((a, b) => Number(b.value) - Number(a.value))
            const sum = array.reduce((s, x) => s + (Number(x.value) || 0), 0)
            array.unshift({ key: t('Total:'), value: formatTokens(sum) })
            return array
          },
        },
      },
      animationAppear: { duration: 500 },
    }
  }, [chartGridColor, chartTextColor, orderedPoints, t])

  return (
    <section className='bg-card overflow-hidden rounded-lg border'>
      <header className='flex items-start justify-between gap-4 px-5 py-4'>
        <div className='min-w-0 flex-1'>
          <h2 className='text-foreground inline-flex items-center gap-2 text-base font-semibold'>
            <User className='text-primary size-4' />
            {t('Top Users')}
          </h2>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(PERIOD_DESCRIPTIONS[props.period])}
          </p>
        </div>
        <div className='shrink-0 text-right'>
          <div className='text-foreground font-mono text-2xl font-semibold tabular-nums'>
            {formatTokens(totalTokens)}
          </div>
          <div className='text-muted-foreground/80 text-[10px] font-medium tracking-widest uppercase'>
            {t('tokens')}
          </div>
        </div>
      </header>

      <div className='px-5 pb-5'>
        <div className='h-60 sm:h-72'>
          {themeReady && spec ? (
            <VChart
              key={`users-history-${resolvedTheme}-${props.period}`}
              spec={{
                ...spec,
                theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                background: 'transparent',
              }}
              option={VCHART_OPTION}
            />
          ) : (
            <div className='text-muted-foreground/80 flex h-full items-center justify-center text-xs'>
              {t('No history data available')}
            </div>
          )}
        </div>
      </div>

      <div className='border-t'>
        <header className='px-5 pt-4 pb-2'>
          <h3 className='text-foreground inline-flex items-center gap-2 text-sm font-semibold'>
            <Trophy className='size-3.5 text-amber-500' />
            {t('User Leaderboard')}
          </h3>
          <p className='text-muted-foreground/80 mt-0.5 text-xs'>
            {t('Compare the most active users by token consumption')}
          </p>
        </header>
        {props.rows.length === 0 ? (
          <div className='text-muted-foreground/80 px-5 py-8 text-center text-sm'>
            {t('No users match the selected filters')}
          </div>
        ) : (
          <div className='px-5 pt-1 pb-4'>
            <UserLeaderboard rows={props.rows} />
          </div>
        )}
      </div>
    </section>
  )
}

type UserLeaderboardProps = {
  rows: UserRanking[]
  limit?: number
}

function UserLeaderboard(props: UserLeaderboardProps) {
  const limited = props.limit ? props.rows.slice(0, props.limit) : props.rows
  const half = Math.ceil(limited.length / 2)
  const left = limited.slice(0, half)
  const right = limited.slice(half)

  if (limited.length === 0) return null

  return (
    <div className='grid grid-cols-1 gap-x-8 md:grid-cols-2'>
      <UserList rows={left} />
      {right.length > 0 && <UserList rows={right} />}
    </div>
  )
}

function UserList(props: { rows: UserRanking[] }) {
  const { t } = useTranslation()
  return (
    <ul>
      {props.rows.map((row) => (
        <li
          key={row.username}
          className='flex items-center gap-3 py-2.5'
        >
          <span className='text-muted-foreground/80 w-6 shrink-0 text-right font-mono text-xs tabular-nums'>
            {row.rank}.
          </span>
          <div className='bg-muted text-muted-foreground flex size-7 shrink-0 items-center justify-center rounded-full font-mono text-xs font-semibold'>
            {row.username.charAt(0).toUpperCase()}
          </div>
          <div className='min-w-0 flex-1'>
            <span className='text-foreground block truncate text-sm font-medium'>
              {row.username}
            </span>
            <span className='text-muted-foreground/80 block truncate text-xs'>
              {t('User ID')}: {row.user_id}
            </span>
          </div>
          <div className='shrink-0 text-right'>
            <div className='text-foreground font-mono text-sm font-semibold tabular-nums'>
              {formatTokens(row.total_tokens)}
              <span className='text-muted-foreground/80 font-normal'>
                {' '}
                {t('tokens')}
              </span>
            </div>
            <GrowthText value={row.growth_pct} className='text-[11px]' />
          </div>
        </li>
      ))}
    </ul>
  )
}
