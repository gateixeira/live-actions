import { useState, useEffect, useCallback, useMemo } from 'react'
import { ThemeProvider, BaseStyles, Box, Header, Text, UnderlineNav, FormControl, Select, SelectPanel, Button } from '@primer/react'
import { MarkGithubIcon, GraphIcon, AlertIcon, ServerIcon } from '@primer/octicons-react'
import { MetricsCards } from './components/MetricsCards'
import { DemandChart } from './components/DemandChart'
import { WorkflowTable } from './components/WorkflowTable'
import { FailureAnalytics } from './components/FailureAnalytics'
import { LabelDemand } from './components/LabelDemand'
import { useSSE } from './hooks/useSSE'
import { getMetrics, getRepositories, initCsrf } from './api/client'
import type { SelectPanelItemInput } from '@primer/react'
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
  const [selectedStatus, setSelectedStatus] = useState('')
  const [repoItems, setRepoItems] = useState<{ id: string; text: string }[]>([])
  const [repoPanelOpen, setRepoPanelOpen] = useState(false)
  const [repoFilter, setRepoFilter] = useState('')
  const STATUS_OPTIONS = [
    { id: '', label: 'All statuses' },
    { id: 'requested', label: 'Requested' },
    { id: 'in_progress', label: 'In Progress' },
    { id: 'success', label: 'Success' },
    { id: 'failure', label: 'Failed' },
    { id: 'cancelled', label: 'Cancelled' },
    { id: 'action_required', label: 'Action Required' },
  ]

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

  const filteredRepoItems = useMemo(() => {
    const allRepos = [{ id: '', text: 'All repositories' }, ...repoItems]
    if (!repoFilter) return allRepos
    const lower = repoFilter.toLowerCase()
    return allRepos.filter((r) => r.text.toLowerCase().includes(lower))
  }, [repoItems, repoFilter])

  const selectedRepoItem = useMemo(
    () => (selectedRepo ? { id: selectedRepo, text: selectedRepo } : { id: '', text: 'All repositories' }),
    [selectedRepo],
  )

  const repoFilterBox = (
    <FormControl>
      <FormControl.Label sx={{ mb: 2 }}>Filter by repository</FormControl.Label>
      <SelectPanel
        title="Filter by repository"
        placeholder="Search repositories..."
        open={repoPanelOpen}
        onOpenChange={(open) => {
          setRepoPanelOpen(open)
          if (!open) setRepoFilter('')
        }}
        items={filteredRepoItems}
        selected={selectedRepoItem}
        onSelectedChange={(item: SelectPanelItemInput | undefined) => {
          if (!item || item.id === '') {
            setSelectedRepo('')
          } else {
            setSelectedRepo(String(item.id))
          }
          setRepoPanelOpen(false)
          setRepoFilter('')
        }}
        filterValue={repoFilter}
        onFilterChange={setRepoFilter}
        renderAnchor={(props) => (
          <Button {...props} sx={{ bg: 'canvas.default', '[data-component="text"]': { textAlign: 'left' } }}>
            {selectedRepo || 'All repositories'}
          </Button>
        )}
        overlayProps={{ width: 'large', sx: { bg: 'canvas.overlay', color: 'fg.default' } }}
        height="medium"
      />
    </FormControl>
  )

  const repoOnlyFilter = (
    <Box sx={{ mb: 3, p: 3, bg: 'canvas.default', borderRadius: 2, borderWidth: 1, borderStyle: 'solid', borderColor: 'border.default' }}>
      {repoFilterBox}
    </Box>
  )

  const dashboardFilter = (
    <Box sx={{ mb: 3, p: 3, bg: 'canvas.default', borderRadius: 2, borderWidth: 1, borderStyle: 'solid', borderColor: 'border.default', display: 'flex', gap: 4, alignItems: 'flex-end' }}>
      {repoFilterBox}
      <FormControl sx={{ flex: 1 }}>
        <FormControl.Label sx={{ mb: 2 }}>Filter by status</FormControl.Label>
        <Select
          value={selectedStatus}
          onChange={(e: React.ChangeEvent<HTMLSelectElement>) => setSelectedStatus(e.target.value)}
          sx={{ bg: 'canvas.default' }}
        >
          {STATUS_OPTIONS.map((opt) => (
            <Select.Option key={opt.id} value={opt.id}>{opt.label}</Select.Option>
          ))}
        </Select>
      </FormControl>
    </Box>
  )

  return (
    <ThemeProvider colorMode="auto">
      <BaseStyles>
        <Box sx={{ minHeight: '100vh', bg: 'canvas.inset', color: 'fg.default' }}>
          <Header>
            <Header.Item>
              <Header.Link href="/" sx={{ fontSize: 2, fontWeight: 'bold', display: 'flex', alignItems: 'center', gap: 2 }}>
                <MarkGithubIcon size={24} />
                Live Actions
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

                {dashboardFilter}
                <WorkflowTable key={`${selectedRepo}:${selectedStatus}`} ready={ready} refreshSignal={workflowRefresh} repo={selectedRepo} status={selectedStatus} />
              </>
            )}

            {activeTab === 'failures' && (
              <>
                {repoOnlyFilter}
                <FailureAnalytics ready={ready} repo={selectedRepo} />
              </>
            )}

            {activeTab === 'labels' && (
              <>
                {repoOnlyFilter}
                <LabelDemand ready={ready} repo={selectedRepo} />
              </>
            )}
          </Box>
        </Box>
      </BaseStyles>
    </ThemeProvider>
  )
}
