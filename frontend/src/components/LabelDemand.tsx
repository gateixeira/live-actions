import { useState, useEffect, useCallback, useMemo } from 'react'
import { Box, Text, SegmentedControl } from '@primer/react'
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
import { Card } from './Card'
import { PERIODS, formatTime, formatSeconds } from '../utils/format'
import { useChartColors } from '../hooks/useChartColors'
import type { LabelDemandResponse, Period } from '../api/types'

interface Props {
  ready: boolean
  repo: string
}

export function LabelDemand({ ready, repo }: Props) {
  const colors = useChartColors()
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
            bg: 'canvas.default',
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
                    stroke={colors.palette[i % colors.palette.length]}
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
            bg: 'canvas.default',
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
                  <td colSpan={5} style={{ textAlign: 'center' }}>
                    <Text sx={{ color: 'fg.muted' }}>No data available for this period</Text>
                  </td>
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
