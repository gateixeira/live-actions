import { useState, useEffect, useCallback, useMemo } from 'react'
import { clsx } from 'clsx'
import {
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts'
import { getRunnerStats } from '../api/client'
import { PERIODS, formatSeconds, relativeTime } from '../utils/format'
import { runnerLabel, sharedRunnerPrefix } from '../utils/runnerLabels'
import type { Period, RunnerStatsResponse } from '../api/types'

// Keep the chart compact on large fleets; the table below lists every runner.
const CHART_LIMIT = 20
const CHART_ROW_HEIGHT = 28
const MAX_RUNNER_LABEL_LEN = 22

const SERIES = [
  { key: 'Succeeded', color: '#34d399' },
  { key: 'Failed', color: '#f87171' },
  { key: 'Cancelled', color: '#6b7280' },
  { key: 'Other', color: '#60a5fa' },
] as const

interface Props {
  ready: boolean
  repo: string
}

export function RunnerStats({ ready, repo }: Props) {
  const [period, setPeriod] = useState<Period>('day')
  const [data, setData] = useState<RunnerStatsResponse | null>(null)

  const load = useCallback((p: Period) => {
    getRunnerStats(p, repo)
      .then(setData)
      .catch((err) => console.error('Failed to load runner stats', err))
  }, [repo])

  useEffect(() => {
    if (!ready) return
    load(period)
    const interval = setInterval(() => load(period), 30_000)
    return () => clearInterval(interval)
  }, [period, load, ready])

  const { chartData, chartRunnerCount, labelPrefix } = useMemo(() => {
    const withJobs = (data?.summary ?? [])
      .filter((s) => s.total_jobs > 0)
      .sort((a, b) => b.total_jobs - a.total_jobs || a.runner_name.localeCompare(b.runner_name))
    const shown = withJobs.slice(0, CHART_LIMIT)
    return {
      chartRunnerCount: withJobs.length,
      labelPrefix: sharedRunnerPrefix(shown.map((s) => s.runner_name)),
      chartData: shown.map((s) => ({
        runner: s.runner_name,
        Succeeded: s.succeeded,
        Failed: s.failed,
        Cancelled: s.cancelled,
        Other: s.other,
      })),
    }
  }, [data])

  const summary = data?.summary ?? []
  const chartHeight = Math.max(chartData.length * CHART_ROW_HEIGHT + 40, 120)

  return (
    <div className="space-y-6">
      {/* Jobs per runner by conclusion */}
      <div className="rounded-xl border border-gray-800 bg-gray-900 p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold text-gray-200">Jobs by Runner</h3>
          <div className="flex rounded-lg border border-gray-700 bg-gray-800 p-0.5">
            {PERIODS.map((p) => (
              <button
                key={p.value}
                onClick={() => {
                  setPeriod(p.value)
                  load(p.value)
                }}
                className={clsx(
                  'rounded-md px-3 py-1.5 text-xs font-medium transition-colors',
                  period === p.value
                    ? 'bg-gray-700 text-white'
                    : 'text-gray-400 hover:text-gray-200',
                )}
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>

        {chartData.length === 0 ? (
          <div className="flex h-[120px] items-center justify-center">
            <span className="text-sm text-gray-600">No completed jobs on any runner in this period</span>
          </div>
        ) : (
          <>
            <div style={{ height: chartHeight }}>
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={chartData} layout="vertical" margin={{ top: 0, right: 16, bottom: 0, left: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#1f2937" horizontal={false} />
                  <XAxis
                    type="number"
                    allowDecimals={false}
                    fontSize={11}
                    tick={{ fill: '#6b7280' }}
                    axisLine={{ stroke: '#374151' }}
                    tickLine={false}
                  />
                  <YAxis
                    type="category"
                    dataKey="runner"
                    width={150}
                    tickFormatter={(name: string) => runnerLabel(name, labelPrefix, MAX_RUNNER_LABEL_LEN)}
                    fontSize={11}
                    tick={{ fill: '#9ca3af' }}
                    axisLine={false}
                    tickLine={false}
                  />
                  <Tooltip
                    cursor={{ fill: '#1f2937' }}
                    contentStyle={{
                      backgroundColor: '#111827',
                      border: '1px solid #374151',
                      borderRadius: '0.5rem',
                      fontSize: '0.75rem',
                    }}
                    itemStyle={{ color: '#e5e7eb' }}
                    labelStyle={{ color: '#9ca3af' }}
                  />
                  {SERIES.map((s) => (
                    <Bar key={s.key} dataKey={s.key} stackId="a" fill={s.color} maxBarSize={18} />
                  ))}
                </BarChart>
              </ResponsiveContainer>
            </div>

            <div className="mt-3 flex flex-wrap items-center justify-center gap-4 text-xs text-gray-400">
              {SERIES.map((s) => (
                <span key={s.key} className="flex items-center gap-1.5">
                  <span className="h-2 w-2 rounded-full" style={{ backgroundColor: s.color }} />
                  {s.key}
                </span>
              ))}
            </div>
            {chartRunnerCount > CHART_LIMIT && (
              <p className="mt-2 text-center text-xs text-gray-600">
                Showing the {CHART_LIMIT} busiest of {chartRunnerCount} runners
              </p>
            )}
          </>
        )}
      </div>

      {/* Runner summary table */}
      <div>
        <h3 className="mb-3 text-sm font-semibold text-gray-200">Runner Summary</h3>
        <div className="overflow-hidden rounded-xl border border-gray-800 bg-gray-900">
          <div className="overflow-x-auto">
          <table className="w-full min-w-[900px] text-left">
            <thead>
              <tr className="border-b border-gray-800 bg-gray-800/40 text-xs font-medium text-gray-500 uppercase tracking-wider">
                <th className="py-3 px-4">Runner</th>
                <th className="py-3 px-4">Group</th>
                <th className="py-3 px-4">Jobs</th>
                <th className="py-3 px-4">Succeeded</th>
                <th className="py-3 px-4">Failed</th>
                <th className="py-3 px-4">Cancelled</th>
                <th className="py-3 px-4">Failure Rate</th>
                <th className="py-3 px-4">Avg Duration</th>
                <th className="py-3 px-4">Running Now</th>
                <th className="py-3 px-4">Last Job</th>
              </tr>
            </thead>
            <tbody>
              {summary.length === 0 ? (
                <tr>
                  <td colSpan={10} className="py-8 text-center text-sm text-gray-600">
                    No runner activity for this period
                  </td>
                </tr>
              ) : (
                summary.map((s) => (
                  <tr key={s.runner_name} className="border-t border-gray-800 hover:bg-gray-800/30">
                    <td className="py-3 px-4">
                      <span className="text-sm font-medium text-gray-200">{s.runner_name}</span>
                    </td>
                    <td className="py-3 px-4 text-sm text-gray-400">{s.runner_group_name || '-'}</td>
                    <td className="py-3 px-4 text-sm tabular-nums text-gray-400">{s.total_jobs}</td>
                    <td className="py-3 px-4 text-sm tabular-nums text-gray-400">{s.succeeded}</td>
                    <td
                      className={clsx(
                        'py-3 px-4 text-sm tabular-nums',
                        s.failed > 0 ? 'text-red-400' : 'text-gray-400',
                      )}
                    >
                      {s.failed}
                    </td>
                    <td className="py-3 px-4 text-sm tabular-nums text-gray-400">{s.cancelled}</td>
                    <td
                      className={clsx(
                        'py-3 px-4 text-sm tabular-nums',
                        s.failure_rate > 0.5 ? 'text-red-400' : 'text-gray-400',
                      )}
                    >
                      {s.total_jobs > 0 ? `${(s.failure_rate * 100).toFixed(1)}%` : '-'}
                    </td>
                    <td className="py-3 px-4 text-sm tabular-nums text-gray-400">
                      {s.total_jobs > 0 ? formatSeconds(s.avg_duration_seconds) : '-'}
                    </td>
                    <td className="py-3 px-4">
                      {s.running > 0 ? (
                        <span className="inline-flex items-center rounded-full bg-emerald-400/10 px-2 py-0.5 text-xs font-medium text-emerald-400">
                          {s.running}
                        </span>
                      ) : (
                        <span className="text-sm text-gray-600">0</span>
                      )}
                    </td>
                    <td
                      className="py-3 px-4 text-sm text-gray-500"
                      title={s.last_job_at ? new Date(s.last_job_at).toLocaleString() : undefined}
                    >
                      {s.last_job_at ? relativeTime(s.last_job_at) : '-'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
          </div>
        </div>
      </div>
    </div>
  )
}
