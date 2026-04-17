import { useMemo } from 'react'
import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts'
import { clsx } from 'clsx'
import type { MetricsResponse, Period } from '../api/types'
import { PERIODS, formatTime } from '../utils/format'

interface Props {
  data: MetricsResponse | null
  period: Period
  onPeriodChange: (p: Period) => void
}

export function DemandChart({ data, period, onPeriodChange }: Props) {
  const chartData = useMemo(() => {
    if (!data) return []
    const running = data.time_series.running_jobs?.data?.result ?? []
    const queued = data.time_series.queued_jobs?.data?.result ?? []

    const map = new Map<number, { ts: number; running: number; queued: number }>()

    for (const series of running) {
      for (const [ts, val] of series.values) {
        const existing = map.get(ts) ?? { ts, running: 0, queued: 0 }
        existing.running += parseFloat(val) || 0
        map.set(ts, existing)
      }
    }
    for (const series of queued) {
      for (const [ts, val] of series.values) {
        const existing = map.get(ts) ?? { ts, running: 0, queued: 0 }
        existing.queued += parseFloat(val) || 0
        map.set(ts, existing)
      }
    }

    return Array.from(map.values()).sort((a, b) => a.ts - b.ts)
  }, [data])

  return (
    <div className="mb-6 rounded-xl border border-gray-800 bg-gray-900 p-5">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-semibold text-gray-200">Runner Demand</h3>
        <div className="flex rounded-lg border border-gray-700 bg-gray-800 p-0.5">
          {PERIODS.map((p) => (
            <button
              key={p.value}
              onClick={() => onPeriodChange(p.value)}
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
        {chartData.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <span className="text-sm text-gray-600">No data available for this period</span>
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={chartData}>
              <defs>
                <linearGradient id="gradRunning" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#34d399" stopOpacity={0.3} />
                  <stop offset="100%" stopColor="#34d399" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="gradQueued" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#fbbf24" stopOpacity={0.3} />
                  <stop offset="100%" stopColor="#fbbf24" stopOpacity={0} />
                </linearGradient>
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
                formatter={(value: number, name: string) => [Math.round(value), name]}
                contentStyle={{
                  backgroundColor: '#111827',
                  border: '1px solid #374151',
                  borderRadius: '0.5rem',
                  fontSize: '0.75rem',
                }}
                itemStyle={{ color: '#e5e7eb' }}
                labelStyle={{ color: '#9ca3af' }}
              />
              <Area
                type="monotone"
                dataKey="running"
                name="Running"
                stroke="#34d399"
                strokeWidth={2}
                fill="url(#gradRunning)"
                dot={false}
              />
              <Area
                type="monotone"
                dataKey="queued"
                name="Queued"
                stroke="#fbbf24"
                strokeWidth={2}
                fill="url(#gradQueued)"
                dot={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Legend */}
      {chartData.length > 0 && (
        <div className="mt-3 flex items-center justify-center gap-6 text-xs text-gray-400">
          <span className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-emerald-400" />
            Running
          </span>
          <span className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-amber-400" />
            Queued
          </span>
        </div>
      )}
    </div>
  )
}
