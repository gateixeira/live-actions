import { useState, useEffect, useCallback } from 'react'
import { ThemeProvider, BaseStyles, Box, Header, Text } from '@primer/react'
import { MarkGithubIcon } from '@primer/octicons-react'
import { MetricsCards } from './components/MetricsCards'
import { DemandChart } from './components/DemandChart'
import { WorkflowTable } from './components/WorkflowTable'
import { useSSE } from './hooks/useSSE'
import { getMetrics, initCsrf } from './api/client'
import type { MetricsResponse, Period } from './api/types'

export default function App() {
  const [period, setPeriod] = useState<Period>('day')
  const [metricsData, setMetricsData] = useState<MetricsResponse | null>(null)
  const [liveRunning, setLiveRunning] = useState<number | null>(null)
  const [liveQueued, setLiveQueued] = useState<number | null>(null)
  const [ready, setReady] = useState(false)

  // Initialize CSRF token before making API calls
  useEffect(() => {
    initCsrf().then(() => setReady(true))
  }, [])

  const loadMetrics = useCallback(
    (p: Period) => {
      getMetrics(p)
        .then(setMetricsData)
        .catch(() => {})
    },
    [],
  )

  useEffect(() => {
    if (!ready) return
    loadMetrics(period)
    const interval = setInterval(() => loadMetrics(period), 30_000)
    return () => clearInterval(interval)
  }, [period, loadMetrics, ready])

  useSSE({
    onMetricsUpdate: (data) => {
      setLiveRunning(data.running_jobs)
      setLiveQueued(data.queued_jobs)
    },
    onWorkflowUpdate: () => {
      // Trigger table refresh via the static ref
      ;(WorkflowTable as any)._refresh?.()
    },
  })

  const running = liveRunning ?? metricsData?.current_metrics?.running_jobs ?? 0
  const queued = liveQueued ?? metricsData?.current_metrics?.queued_jobs ?? 0
  const avgQueueTime = metricsData?.current_metrics?.avg_queue_time ?? 0
  const peakDemand = metricsData?.current_metrics?.peak_demand ?? 0

  return (
    <ThemeProvider colorMode="auto">
      <BaseStyles>
        <Box sx={{ minHeight: '100vh', bg: 'canvas.default', color: 'fg.default' }}>
          <Header>
            <Header.Item>
              <Header.Link href="/" sx={{ fontSize: 2, fontWeight: 'bold', display: 'flex', alignItems: 'center', gap: 2 }}>
                <MarkGithubIcon size={24} />
                <span>Live Actions</span>
              </Header.Link>
            </Header.Item>
            <Header.Item full />
            <Header.Item>
              <Text sx={{ fontSize: 0, color: 'header.text' }}>Runner Monitoring</Text>
            </Header.Item>
          </Header>

          <Box sx={{ maxWidth: 1280, mx: 'auto', px: [3, 4], py: 4 }}>
            <MetricsCards
              running={running}
              queued={queued}
              avgQueueTime={avgQueueTime}
              peakDemand={peakDemand}
            />

            <DemandChart
              data={metricsData}
              period={period}
              onPeriodChange={(p) => {
                setPeriod(p)
                loadMetrics(p)
              }}
            />

            <WorkflowTable />
          </Box>
        </Box>
      </BaseStyles>
    </ThemeProvider>
  )
}
