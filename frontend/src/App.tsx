import { useState, useEffect, useCallback } from 'react'
import { ThemeProvider, BaseStyles, Box, Header, Text, UnderlineNav, Autocomplete, FormControl } from '@primer/react'
import { MarkGithubIcon, GraphIcon, AlertIcon, ServerIcon } from '@primer/octicons-react'
import { MetricsCards } from './components/MetricsCards'
import { DemandChart } from './components/DemandChart'
import { WorkflowTable } from './components/WorkflowTable'
import { FailureAnalytics } from './components/FailureAnalytics'
import { LabelDemand } from './components/LabelDemand'
import { useSSE } from './hooks/useSSE'
import { getMetrics, getRepositories, initCsrf } from './api/client'
import type { MetricsResponse, Period } from './api/types'

type Tab = 'dashboard' | 'failures' | 'labels'

export default function App() {
  const [activeTab, setActiveTab] = useState<Tab>('dashboard')
  const [period, setPeriod] = useState<Period>('day')
  const [metricsData, setMetricsData] = useState<MetricsResponse | null>(null)
  const [liveRunning, setLiveRunning] = useState<number | null>(null)
  const [liveQueued, setLiveQueued] = useState<number | null>(null)
  const [ready, setReady] = useState(false)
  const [selectedRepo, setSelectedRepo] = useState('')
  const [repoItems, setRepoItems] = useState<{ id: string; text: string }[]>([])

  // Initialize CSRF token before making API calls
  useEffect(() => {
    initCsrf().then(() => setReady(true))
  }, [])

  // Fetch repository list for autocomplete
  useEffect(() => {
    if (!ready) return
    getRepositories()
      .then((r) => setRepoItems(
        r.repositories.map((name) => ({ id: name, text: name }))
      ))
      .catch((err) => console.error('Failed to load repositories', err))
  }, [ready])

  const loadMetrics = useCallback(
    (p: Period) => {
      getMetrics(p)
        .then(setMetricsData)
        .catch((err) => console.error('Failed to load metrics', err))
    },
    [],
  )

  useEffect(() => {
    if (!ready) return
    loadMetrics(period)
    const interval = setInterval(() => loadMetrics(period), 30_000)
    return () => clearInterval(interval)
  }, [period, loadMetrics, ready])

  const [workflowRefresh, setWorkflowRefresh] = useState(0)

  useSSE({
    onMetricsUpdate: (data) => {
      setLiveRunning(data.running_jobs)
      setLiveQueued(data.queued_jobs)
    },
    onWorkflowUpdate: () => {
      setWorkflowRefresh((r) => r + 1)
    },
  })

  const running = liveRunning ?? metricsData?.current_metrics?.running_jobs ?? 0
  const queued = liveQueued ?? metricsData?.current_metrics?.queued_jobs ?? 0
  const avgQueueTime = metricsData?.current_metrics?.avg_queue_time ?? 0
  const peakDemand = metricsData?.current_metrics?.peak_demand ?? 0

  const repoFilter = (
    <Box sx={{ mb: 3, p: 3, bg: 'canvas.subtle', borderRadius: 2, borderWidth: 1, borderStyle: 'solid', borderColor: 'border.default' }}>
      <FormControl>
        <FormControl.Label sx={{ mb: 2 }}>Filter by repository</FormControl.Label>
        <Autocomplete id="repo-filter">
          <Autocomplete.Input
            placeholder="All repositories"
            value={selectedRepo}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
              if (e.target.value === '') setSelectedRepo('')
            }}
            sx={{ bg: 'canvas.default', width: '100%' }}
          />
          <Autocomplete.Overlay sx={{ zIndex: 100, bg: 'canvas.overlay', borderColor: 'border.default', boxShadow: 'shadow.large' }}>
            <Autocomplete.Menu
              items={repoItems}
              selectedItemIds={selectedRepo ? [selectedRepo] : []}
              onSelectedChange={(items) => {
                if (!items) {
                  setSelectedRepo('')
                } else if (Array.isArray(items)) {
                  setSelectedRepo(items.length > 0 ? (items[items.length - 1].text ?? '') : '')
                } else {
                  setSelectedRepo(items.text ?? '')
                }
              }}
              selectionVariant="single"
              aria-labelledby="repo-filter-label"
            />
          </Autocomplete.Overlay>
        </Autocomplete>
      </FormControl>
    </Box>
  )

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
            <UnderlineNav aria-label="Main navigation" sx={{ mb: 4 }}>
              <UnderlineNav.Item
                aria-current={activeTab === 'dashboard' ? 'page' : undefined}
                onSelect={() => setActiveTab('dashboard')}
                icon={GraphIcon}
              >
                Dashboard
              </UnderlineNav.Item>
              <UnderlineNav.Item
                aria-current={activeTab === 'failures' ? 'page' : undefined}
                onSelect={() => setActiveTab('failures')}
                icon={AlertIcon}
              >
                Failure Analytics
              </UnderlineNav.Item>
              <UnderlineNav.Item
                aria-current={activeTab === 'labels' ? 'page' : undefined}
                onSelect={() => setActiveTab('labels')}
                icon={ServerIcon}
              >
                Runner Labels
              </UnderlineNav.Item>
            </UnderlineNav>

            {activeTab === 'dashboard' && (
              <>
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

                {repoFilter}
                <WorkflowTable ready={ready} refreshSignal={workflowRefresh} repo={selectedRepo} />
              </>
            )}

            {activeTab === 'failures' && (
              <>
                {repoFilter}
                <FailureAnalytics ready={ready} repo={selectedRepo} />
              </>
            )}

            {activeTab === 'labels' && (
              <>
                {repoFilter}
                <LabelDemand ready={ready} repo={selectedRepo} />
              </>
            )}
          </Box>
        </Box>
      </BaseStyles>
    </ThemeProvider>
  )
}
