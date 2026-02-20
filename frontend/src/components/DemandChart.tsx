import { useMemo } from 'react'
import { Box, SegmentedControl, Text } from '@primer/react'
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
  CartesianGrid,
} from 'recharts'
import type { MetricsResponse, Period } from '../api/types'

interface Props {
  data: MetricsResponse | null
  period: Period
  onPeriodChange: (p: Period) => void
}

const PERIODS: { label: string; value: Period }[] = [
  { label: '1h', value: 'hour' },
  { label: '1d', value: 'day' },
  { label: '1w', value: 'week' },
  { label: '1m', value: 'month' },
]

function formatTime(ts: number, period: Period): string {
  const d = new Date(ts * 1000)
  if (period === 'hour') return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (period === 'day') return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

export function DemandChart({ data, period, onPeriodChange }: Props) {
  const chartData = useMemo(() => {
    if (!data) return []
    const running = data.time_series.running_jobs?.data?.result ?? []
    const queued = data.time_series.queued_jobs?.data?.result ?? []

    // Build a timestamp → values map
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

  const selectedIndex = PERIODS.findIndex((p) => p.value === period)

  return (
    <Box sx={{ mb: 4 }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
        <Text sx={{ fontSize: 1, fontWeight: 'bold' }}>Runner Demand</Text>
        <SegmentedControl
          aria-label="Time period"
          onChange={(i) => onPeriodChange(PERIODS[i].value)}
        >
          {PERIODS.map((p, i) => (
            <SegmentedControl.Button key={p.value} selected={i === selectedIndex}>
              {p.label}
            </SegmentedControl.Button>
          ))}
        </SegmentedControl>
      </Box>

      <Box
        sx={{
          height: 350,
          border: '1px solid',
          borderColor: 'border.default',
          borderRadius: 2,
          p: 3,
          bg: 'canvas.default',
        }}
      >
        {chartData.length === 0 ? (
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
            <Text sx={{ color: 'fg.muted' }}>No data available for this period</Text>
          </Box>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" opacity={0.3} />
              <XAxis
                dataKey="ts"
                tickFormatter={(v) => formatTime(v, period)}
                fontSize={11}
              />
              <YAxis allowDecimals={false} fontSize={11} />
              <Tooltip
                labelFormatter={(v) => new Date((v as number) * 1000).toLocaleString()}
                formatter={(value: number, name: string) => [Math.round(value), name]}
              />
              <Legend />
              <Line
                type="monotone"
                dataKey="running"
                name="Running"
                stroke="#2da44e"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="queued"
                name="Queued"
                stroke="#bf8700"
                strokeWidth={2}
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        )}
      </Box>
    </Box>
  )
}
