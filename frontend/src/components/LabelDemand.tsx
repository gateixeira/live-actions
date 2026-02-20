import { useState, useEffect, useCallback, useMemo } from 'react'
import { Box, Heading, Text, SegmentedControl } from '@primer/react'
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
import { getLabelDemand } from '../api/client'
import type { LabelDemandResponse, Period } from '../api/types'

interface Props {
  ready: boolean
  repo: string
}

const PERIODS: { label: string; value: Period }[] = [
  { label: '1h', value: 'hour' },
  { label: '1d', value: 'day' },
  { label: '1w', value: 'week' },
  { label: '1m', value: 'month' },
]

// Distinct colors for up to 10 labels
const LABEL_COLORS = [
  '#2da44e', '#0969da', '#bf8700', '#cf222e', '#8250df',
  '#1a7f37', '#0550ae', '#953800', '#a40e26', '#6639ba',
]

function formatTime(ts: number, period: Period): string {
  const d = new Date(ts * 1000)
  if (period === 'hour' || period === 'day')
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

function formatSeconds(s: number): string {
  if (!s || s === 0) return '0s'
  const total = Math.round(s)
  if (total < 60) return `${total}s`
  const m = Math.floor(total / 60)
  const rem = total % 60
  if (m < 60) return rem > 0 ? `${m}m ${rem}s` : `${m}m`
  const h = Math.floor(m / 60)
  const rm = m % 60
  return rm > 0 ? `${h}h ${rm}m` : `${h}h`
}

function Card({ label, value, sub }: { label: string; value: React.ReactNode; sub?: string }) {
  return (
    <Box
      sx={{
        p: 3,
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'border.default',
        bg: 'canvas.subtle',
        minWidth: 160,
        flex: '1 1 200px',
      }}
    >
      <Text sx={{ fontSize: 0, color: 'fg.muted', display: 'block', mb: 1 }}>{label}</Text>
      <Heading as="h3" sx={{ fontSize: 4, fontWeight: 'bold' }}>
        {value}
      </Heading>
      {sub && (
        <Text sx={{ fontSize: 0, color: 'fg.muted', display: 'block', mt: 1 }}>{sub}</Text>
      )}
    </Box>
  )
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

  // Get unique labels from summary for consistent ordering/coloring
  const labels = useMemo(() => {
    if (!data?.summary) return []
    return data.summary.map((s) => s.label)
  }, [data])

  // Transform trend data: pivot from [{timestamp, label, count}] to
  // [{ts, "ubuntu-latest": N, "self-hosted": N, ...}]
  // Backfill 0 for labels missing from a bucket so recharts draws continuous lines.
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

  const selectedIndex = PERIODS.findIndex((p) => p.value === period)
  const summary = data?.summary ?? []

  // Active labels (currently running or queued)
  const activeLabels = summary.filter((s) => s.running > 0 || s.queued > 0)

  return (
    <Box>
      {/* Current demand per label */}
      {activeLabels.length > 0 && (
        <Box sx={{ mb: 4 }}>
          <Text sx={{ fontSize: 1, fontWeight: 'bold', mb: 3, display: 'block' }}>Current Demand</Text>
          <Box sx={{ display: 'flex', gap: 3, flexWrap: 'wrap' }}>
            {activeLabels.map((s) => (
              <Card
                key={s.label}
                label={s.label}
                value={`${s.running + s.queued}`}
                sub={`${s.running} running · ${s.queued} queued`}
              />
            ))}
          </Box>
        </Box>
      )}

      {/* Demand trend chart */}
      <Box sx={{ mb: 4 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Text sx={{ fontSize: 1, fontWeight: 'bold' }}>Job Volume by Label</Text>
          <SegmentedControl
            aria-label="Time period"
            onChange={(i) => {
              setPeriod(PERIODS[i].value)
              load(PERIODS[i].value)
            }}
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
            bg: 'canvas.subtle',
          }}
        >
          {trendData.length === 0 ? (
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
              <Text sx={{ color: 'fg.muted' }}>No data available for this period</Text>
            </Box>
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={trendData}>
                <CartesianGrid strokeDasharray="3 3" opacity={0.3} />
                <XAxis
                  dataKey="ts"
                  tickFormatter={(v) => formatTime(v, period)}
                  fontSize={11}
                />
                <YAxis allowDecimals={false} fontSize={11} />
                <Tooltip
                  labelFormatter={(v) => new Date((v as number) * 1000).toLocaleString()}
                />
                <Legend />
                {labels.map((label, i) => (
                  <Line
                    key={label}
                    type="monotone"
                    dataKey={label}
                    stroke={LABEL_COLORS[i % LABEL_COLORS.length]}
                    strokeWidth={2}
                    dot={false}
                  />
                ))}
              </LineChart>
            </ResponsiveContainer>
          )}
        </Box>
      </Box>

      {/* Label summary table */}
      <Box sx={{ mb: 4 }}>
        <Text sx={{ fontSize: 1, fontWeight: 'bold', mb: 3, display: 'block' }}>Label Summary</Text>
        <Box
          sx={{
            border: '1px solid',
            borderColor: 'border.default',
            borderRadius: 2,
            overflow: 'hidden',
          }}
        >
          <Box
            as="table"
            sx={{
              width: '100%',
              borderCollapse: 'collapse',
              '& th, & td': { px: 3, py: 2, textAlign: 'left', fontSize: 1 },
              '& th': { bg: 'canvas.subtle', fontWeight: 'bold', borderBottom: '1px solid', borderColor: 'border.default' },
              '& tr:not(:last-child) td': { borderBottom: '1px solid', borderColor: 'border.muted' },
            }}
          >
            <thead>
              <tr>
                <th>Label</th>
                <th>Total Jobs</th>
                <th>Running</th>
                <th>Queued</th>
                <th>Avg Queue Time</th>
              </tr>
            </thead>
            <tbody>
              {summary.length === 0 ? (
                <tr>
                  <Box as="td" colSpan={5} sx={{ textAlign: 'center', color: 'fg.muted', py: 4 }}>
                    No data available for this period
                  </Box>
                </tr>
              ) : (
                summary.map((s) => (
                  <tr key={s.label}>
                    <td>
                      <Text sx={{ fontWeight: 'semibold' }}>{s.label}</Text>
                    </td>
                    <td>{s.total_jobs}</td>
                    <td>{s.running}</td>
                    <td>{s.queued}</td>
                    <td>{formatSeconds(s.avg_queue_seconds)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </Box>
        </Box>
      </Box>
    </Box>
  )
}
