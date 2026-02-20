import { useState, useEffect, useCallback, useMemo } from 'react'
import { Box, Heading, Text, SegmentedControl, Label } from '@primer/react'
import {
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
  CartesianGrid,
} from 'recharts'
import { getFailureAnalytics } from '../api/client'
import type { FailureAnalyticsResponse, Period } from '../api/types'

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

function formatTime(ts: number, period: Period): string {
  const d = new Date(ts * 1000)
  if (period === 'hour' || period === 'day')
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

function Card({ label, value, color }: { label: string; value: React.ReactNode; color?: string }) {
  return (
    <Box
      sx={{
        p: 3,
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'border.default',
        bg: 'canvas.default',
        flex: 1,
        minWidth: 160,
      }}
    >
      <Text sx={{ fontSize: 0, color: 'fg.muted', display: 'block', mb: 1 }}>{label}</Text>
      <Heading as="h3" sx={{ fontSize: 4, fontWeight: 'bold', color: color || 'fg.default' }}>
        {value}
      </Heading>
    </Box>
  )
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

  const selectedIndex = PERIODS.findIndex((p) => p.value === period)
  const summary = data?.summary

  return (
    <Box>
      {/* Summary Cards */}
      <Box sx={{ display: 'flex', gap: 3, flexWrap: 'wrap', mb: 4 }}>
        <Card label="Total Completed" value={summary?.total_completed ?? 0} />
        <Card
          label="Total Failures"
          value={summary?.total_failed ?? 0}
          color={summary?.total_failed ? 'danger.fg' : undefined}
        />
        <Card
          label="Failure Rate"
          value={`${(summary?.failure_rate ?? 0).toFixed(1)}%`}
          color={summary?.failure_rate && summary.failure_rate > 10 ? 'danger.fg' : undefined}
        />
        <Card label="Cancelled" value={summary?.total_cancelled ?? 0} />
      </Box>

      {/* Failure Trend Chart */}
      <Box sx={{ mb: 4 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Text sx={{ fontSize: 1, fontWeight: 'bold' }}>Failure Trend</Text>
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
              <BarChart data={trendData}>
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
                <Bar dataKey="Successes" stackId="a" fill="#2da44e" />
                <Bar dataKey="Failures" stackId="a" fill="#cf222e" />
                <Bar dataKey="Cancelled" stackId="a" fill="#bf8700" />
              </BarChart>
            </ResponsiveContainer>
          )}
        </Box>
      </Box>

      {/* Top Failing Jobs Table */}
      <Box sx={{ mb: 4 }}>
        <Text sx={{ fontSize: 1, fontWeight: 'bold', mb: 3, display: 'block' }}>Top Failing Jobs</Text>
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
                <th>Job Name</th>
                <th>Failures</th>
                <th>Total Runs</th>
                <th>Failure Rate</th>
              </tr>
            </thead>
            <tbody>
              {(!summary?.top_failing_jobs || summary.top_failing_jobs.length === 0) ? (
                <tr>
                  <Box as="td" colSpan={4} sx={{ textAlign: 'center', color: 'fg.muted', py: 4 }}>
                    No failures in this period
                  </Box>
                </tr>
              ) : (
                summary.top_failing_jobs.map((job) => (
                  <tr key={job.name}>
                    <td>
                      <Text
                        as="a"
                        href={job.html_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        sx={{ fontWeight: 'semibold', color: 'accent.fg' }}
                      >
                        {job.name}
                      </Text>
                    </td>
                    <td>
                      <Label variant="danger">{job.failures}</Label>
                    </td>
                    <td>{job.total}</td>
                    <td>
                      <Text sx={{ color: job.failure_rate > 50 ? 'danger.fg' : 'fg.default' }}>
                        {job.failure_rate.toFixed(1)}%
                      </Text>
                    </td>
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
