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
import { ExternalLink } from 'lucide-react'
import { getFailureAnalytics } from '../api/client'
import { Card } from './Card'
import { PERIODS, formatTime } from '../utils/format'
import type { FailureAnalyticsResponse, Period } from '../api/types'

interface Props {
  ready: boolean
  repo: string
}

export function FailureAnalytics({ ready, repo }: Props) {
  const [period, setPeriod] = useState<Period>('day')
  const [data, setData] = useState<FailureAnalyticsResponse | null>(null)

  const load = useCallback(
    (p: Period) => {
      getFailureAnalytics(p, repo)
        .then(setData)
        .catch((err) => console.error('Failed to load failure analytics', err))
    },
    [repo],
  )

  useEffect(() => {
    if (!ready) return
    load(period)
    const interval = setInterval(() => load(period), 30_000)
    return () => clearInterval(interval)
  }, [period, load, ready])

  const trendData = useMemo(() => {
    if (!data?.trend) return []
    return data.trend.map((p) => ({
      ts: p.timestamp,
      Failures: p.failures,
      Successes: p.successes,
      Cancelled: p.cancelled,
    }))
  }, [data])

  const summary = data?.summary

  return (
    <div className="space-y-6">
      {/* Summary Cards */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Card label="Total Completed" value={summary?.total_completed ?? 0} />
        <Card
          label="Total Failures"
          value={summary?.total_failed ?? 0}
          accent={summary?.total_failed ? 'red' : 'default'}
        />
        <Card
          label="Failure Rate"
          value={`${(summary?.failure_rate ?? 0).toFixed(1)}%`}
          accent={summary?.failure_rate && summary.failure_rate > 10 ? 'red' : 'default'}
        />
        <Card label="Cancelled" value={summary?.total_cancelled ?? 0} />
      </div>

      {/* Failure Trend Chart */}
      <div className="rounded-xl border border-gray-800 bg-gray-900 p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold text-gray-200">Failure Trend</h3>
          <div className="flex rounded-lg border border-gray-700 bg-gray-800 p-0.5">
            {PERIODS.map((p) => (
              <button
                key={p.value}
                onClick={() => {
                  setPeriod(p.value)
                  load(p.value)
                }}
                className={clsx(
                  'rounded-md px-3 py-1 text-xs font-medium transition-colors',
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

        <div className="h-[320px]">
          {trendData.length === 0 ? (
            <div className="flex h-full items-center justify-center">
              <span className="text-sm text-gray-600">No data available for this period</span>
            </div>
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trendData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#1f2937" />
                <XAxis
                  dataKey="ts"
                  tickFormatter={(v) => formatTime(v, period)}
                  fontSize={11}
                  tick={{ fill: '#6b7280' }}
                  axisLine={{ stroke: '#374151' }}
                  tickLine={false}
                />
                <YAxis
                  allowDecimals={false}
                  fontSize={11}
                  tick={{ fill: '#6b7280' }}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip
                  labelFormatter={(v) => new Date((v as number) * 1000).toLocaleString()}
                  contentStyle={{
                    backgroundColor: '#111827',
                    border: '1px solid #374151',
                    borderRadius: '0.5rem',
                    fontSize: '0.75rem',
                  }}
                  itemStyle={{ color: '#e5e7eb' }}
                  labelStyle={{ color: '#9ca3af' }}
                />
                <Bar dataKey="Successes" stackId="a" fill="#34d399" radius={[0, 0, 0, 0]} />
                <Bar dataKey="Failures" stackId="a" fill="#f87171" radius={[0, 0, 0, 0]} />
                <Bar dataKey="Cancelled" stackId="a" fill="#6b7280" radius={[2, 2, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>

        {trendData.length > 0 && (
          <div className="mt-3 flex items-center justify-center gap-6 text-xs text-gray-400">
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full bg-emerald-400" />
              Successes
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full bg-red-400" />
              Failures
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full bg-gray-400" />
              Cancelled
            </span>
          </div>
        )}
      </div>

      {/* Top Failing Jobs Table */}
      <div>
        <h3 className="mb-3 text-sm font-semibold text-gray-200">Top Failing Jobs</h3>
        <div className="overflow-hidden rounded-xl border border-gray-800 bg-gray-900">
          <table className="w-full text-left">
            <thead>
              <tr className="border-b border-gray-800 bg-gray-800/40 text-xs font-medium text-gray-500 uppercase tracking-wider">
                <th className="py-3 px-4">Job Name</th>
                <th className="py-3 px-4">Failures</th>
                <th className="py-3 px-4">Total Runs</th>
                <th className="py-3 px-4">Failure Rate</th>
              </tr>
            </thead>
            <tbody>
              {(!summary?.top_failing_jobs || summary.top_failing_jobs.length === 0) ? (
                <tr>
                  <td colSpan={4} className="py-8 text-center text-sm text-gray-600">
                    No failures in this period
                  </td>
                </tr>
              ) : (
                summary.top_failing_jobs.map((job) => (
                  <tr key={job.name} className="border-t border-gray-800 hover:bg-gray-800/30">
                    <td className="py-3 px-4 text-sm">
                      <a
                        href={job.html_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1 text-indigo-400 hover:text-indigo-300"
                      >
                        {job.name}
                        <ExternalLink className="h-3 w-3" />
                      </a>
                    </td>
                    <td className="py-3 px-4">
                      <span className="inline-flex items-center rounded-full bg-red-400/10 px-2.5 py-0.5 text-xs font-medium text-red-400">
                        {job.failures}
                      </span>
                    </td>
                    <td className="py-3 px-4 text-sm tabular-nums text-gray-400">{job.total}</td>
                    <td className="py-3 px-4">
                      <div className="flex items-center gap-2">
                        <div className="h-1.5 w-16 overflow-hidden rounded-full bg-gray-800">
                          <div
                            className="h-full rounded-full bg-red-400"
                            style={{ width: `${Math.min(job.failure_rate, 100)}%` }}
                          />
                        </div>
                        <span className={clsx(
                          'text-xs tabular-nums font-medium',
                          job.failure_rate > 50 ? 'text-red-400' : 'text-gray-400',
                        )}>
                          {job.failure_rate.toFixed(1)}%
                        </span>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
