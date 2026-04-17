import { useState, useEffect, useCallback, useMemo } from 'react'
import { clsx } from 'clsx'
import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts'
import { getLabelDemand } from '../api/client'
import { Card } from './Card'
import { PERIODS, formatTime, formatSeconds } from '../utils/format'
import type { LabelDemandResponse, Period } from '../api/types'

const CHART_COLORS = [
  '#34d399', '#60a5fa', '#fbbf24', '#f87171', '#a78bfa',
  '#22d3ee', '#f472b6', '#a3e635', '#fb923c', '#2dd4bf',
]

interface Props {
  ready: boolean
  repo: string
}

export function LabelDemand({ ready, repo }: Props) {
  const [period, setPeriod] = useState<Period>('day')
  const [data, setData] = useState<LabelDemandResponse | null>(null)

  const load = useCallback((p: Period) => {
    getLabelDemand(p, repo)
      .then(setData)
      .catch((err) => console.error('Failed to load label demand', err))
  }, [repo])

  useEffect(() => {
    if (!ready) return
    load(period)
    const interval = setInterval(() => load(period), 30_000)
    return () => clearInterval(interval)
  }, [period, load, ready])

  const labels = useMemo(() => {
    if (!data?.summary) return []
    return data.summary.map((s) => s.label)
  }, [data])

  const trendData = useMemo(() => {
    if (!data?.trend || !labels.length) return []
    const map = new Map<number, Record<string, number>>()
    for (const p of data.trend) {
      const existing = map.get(p.timestamp) ?? { ts: p.timestamp }
      existing[p.label] = (existing[p.label] ?? 0) + p.count
      map.set(p.timestamp, existing)
    }
    const rows = Array.from(map.values())
    for (const row of rows) {
      for (const l of labels) {
        if (!(l in row)) row[l] = 0
      }
    }
    return rows.sort((a, b) => a.ts - b.ts)
  }, [data, labels])

  const summary = data?.summary ?? []
  const activeLabels = summary.filter((s) => s.running > 0 || s.queued > 0)

  return (
    <div className="space-y-6">
      {/* Current demand per label */}
      {activeLabels.length > 0 && (
        <div>
          <h3 className="mb-3 text-sm font-semibold text-gray-200">Current Demand</h3>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
            {activeLabels.map((s) => (
              <Card
                key={s.label}
                label={s.label}
                value={`${s.running + s.queued}`}
                sub={`${s.running} running · ${s.queued} queued`}
              />
            ))}
          </div>
        </div>
      )}

      {/* Demand trend chart */}
      <div className="rounded-xl border border-gray-800 bg-gray-900 p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold text-gray-200">Job Volume by Label</h3>
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
              <AreaChart data={trendData}>
                <defs>
                  {labels.map((label, i) => (
                    <linearGradient key={label} id={`grad-${i}`} x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={CHART_COLORS[i % CHART_COLORS.length]} stopOpacity={0.2} />
                      <stop offset="100%" stopColor={CHART_COLORS[i % CHART_COLORS.length]} stopOpacity={0} />
                    </linearGradient>
                  ))}
                </defs>
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
                {labels.map((label, i) => (
                  <Area
                    key={label}
                    type="monotone"
                    dataKey={label}
                    stroke={CHART_COLORS[i % CHART_COLORS.length]}
                    strokeWidth={2}
                    fill={`url(#grad-${i})`}
                    dot={false}
                  />
                ))}
              </AreaChart>
            </ResponsiveContainer>
          )}
        </div>

        {labels.length > 0 && trendData.length > 0 && (
          <div className="mt-3 flex flex-wrap items-center justify-center gap-4 text-xs text-gray-400">
            {labels.map((label, i) => (
              <span key={label} className="flex items-center gap-1.5">
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ backgroundColor: CHART_COLORS[i % CHART_COLORS.length] }}
                />
                {label}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Label summary table */}
      <div>
        <h3 className="mb-3 text-sm font-semibold text-gray-200">Label Summary</h3>
        <div className="overflow-hidden rounded-xl border border-gray-800 bg-gray-900">
          <table className="w-full text-left">
            <thead>
              <tr className="border-b border-gray-800 bg-gray-800/40 text-xs font-medium text-gray-500 uppercase tracking-wider">
                <th className="py-3 px-4">Label</th>
                <th className="py-3 px-4">Total Jobs</th>
                <th className="py-3 px-4">Running</th>
                <th className="py-3 px-4">Queued</th>
                <th className="py-3 px-4">Avg Queue Time</th>
              </tr>
            </thead>
            <tbody>
              {summary.length === 0 ? (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-sm text-gray-600">
                    No data available for this period
                  </td>
                </tr>
              ) : (
                summary.map((s) => (
                  <tr key={s.label} className="border-t border-gray-800 hover:bg-gray-800/30">
                    <td className="py-3 px-4">
                      <span className="text-sm font-medium text-gray-200">{s.label}</span>
                    </td>
                    <td className="py-3 px-4 text-sm tabular-nums text-gray-400">{s.total_jobs}</td>
                    <td className="py-3 px-4">
                      {s.running > 0 ? (
                        <span className="inline-flex items-center rounded-full bg-emerald-400/10 px-2 py-0.5 text-xs font-medium text-emerald-400">
                          {s.running}
                        </span>
                      ) : (
                        <span className="text-sm text-gray-600">0</span>
                      )}
                    </td>
                    <td className="py-3 px-4">
                      {s.queued > 0 ? (
                        <span className="inline-flex items-center rounded-full bg-amber-400/10 px-2 py-0.5 text-xs font-medium text-amber-400">
                          {s.queued}
                        </span>
                      ) : (
                        <span className="text-sm text-gray-600">0</span>
                      )}
                    </td>
                    <td className="py-3 px-4 text-sm tabular-nums text-gray-400">
                      {formatSeconds(s.avg_queue_seconds)}
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
