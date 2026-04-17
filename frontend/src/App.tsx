import { useState, useEffect, useCallback, useMemo } from 'react'
import { Search, ChevronDown } from 'lucide-react'
import { MetricsCards } from './components/MetricsCards'
import { DemandChart } from './components/DemandChart'
import { WorkflowTable } from './components/WorkflowTable'
import { FailureAnalytics } from './components/FailureAnalytics'
import { LabelDemand } from './components/LabelDemand'
import { Sidebar } from './components/Sidebar'
import { useSSE } from './hooks/useSSE'
import { getMetrics, getRepositories, initCsrf } from './api/client'
import type { MetricsResponse, Period } from './api/types'

type Page = 'dashboard' | 'failures' | 'labels'

const PAGE_TITLES: Record<Page, string> = {
  dashboard: 'Dashboard',
  failures: 'Failure Analytics',
  labels: 'Runner Labels',
}

const STATUS_OPTIONS = [
  { id: '', label: 'All statuses' },
  { id: 'requested', label: 'Requested' },
  { id: 'in_progress', label: 'In Progress' },
  { id: 'queued', label: 'Queued' },
  { id: 'stale', label: 'Stale' },
  { id: 'success', label: 'Success' },
  { id: 'failure', label: 'Failed' },
  { id: 'cancelled', label: 'Cancelled' },
  { id: 'action_required', label: 'Action Required' },
]

export default function App() {
  const [activePage, setActivePage] = useState<Page>('dashboard')
  const [period, setPeriod] = useState<Period>('day')
  const [metricsData, setMetricsData] = useState<MetricsResponse | null>(null)
  const [liveRunning, setLiveRunning] = useState<number | null>(null)
  const [liveQueued, setLiveQueued] = useState<number | null>(null)
  const [ready, setReady] = useState(false)
  const [selectedRepo, setSelectedRepo] = useState('')
  const [selectedStatus, setSelectedStatus] = useState('')
  const [repos, setRepos] = useState<string[]>([])
  const [repoSearchOpen, setRepoSearchOpen] = useState(false)
  const [repoSearch, setRepoSearch] = useState('')

  useEffect(() => {
    initCsrf().then(() => setReady(true))
  }, [])

  useEffect(() => {
    if (!ready) return
    getRepositories()
      .then((r) => setRepos(r.repositories))
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

  const { connected } = useSSE({
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
  const avgRunTime = metricsData?.current_metrics?.avg_run_time ?? 0
  const peakDemand = metricsData?.current_metrics?.peak_demand ?? 0

  const filteredRepos = useMemo(() => {
    if (!repoSearch) return repos
    const lower = repoSearch.toLowerCase()
    return repos.filter((r) => r.toLowerCase().includes(lower))
  }, [repos, repoSearch])

  return (
    <div className="flex min-h-screen">
      <Sidebar activePage={activePage} onNavigate={setActivePage} connected={connected} />

      {/* Main content */}
      <main className="ml-56 flex-1 min-h-screen">
        {/* Page header */}
        <header className="sticky top-0 z-20 flex h-14 items-center justify-between border-b border-gray-800 bg-gray-950/80 px-6 backdrop-blur-sm">
          <h1 className="text-base font-semibold text-white">{PAGE_TITLES[activePage]}</h1>

          {/* Filters */}
          <div className="flex items-center gap-3">
            {/* Repository filter */}
            <div className="relative">
              <button
                onClick={() => setRepoSearchOpen(!repoSearchOpen)}
                className="flex items-center gap-2 rounded-lg border border-gray-700 bg-gray-800 px-3 py-1.5 text-xs text-gray-300 hover:border-gray-600 transition-colors"
              >
                <Search className="h-3.5 w-3.5 text-gray-500" />
                <span className="max-w-[140px] truncate">{selectedRepo || 'All repositories'}</span>
                <ChevronDown className="h-3 w-3 text-gray-500" />
              </button>

              {repoSearchOpen && (
                <>
                  <div className="fixed inset-0 z-10" onClick={() => { setRepoSearchOpen(false); setRepoSearch('') }} />
                  <div className="absolute right-0 top-full z-20 mt-1 w-64 overflow-hidden rounded-lg border border-gray-700 bg-gray-800 shadow-xl">
                    <div className="border-b border-gray-700 p-2">
                      <input
                        type="text"
                        autoFocus
                        placeholder="Search repositories..."
                        value={repoSearch}
                        onChange={(e) => setRepoSearch(e.target.value)}
                        className="w-full rounded-md border border-gray-600 bg-gray-900 px-3 py-1.5 text-xs text-gray-200 placeholder-gray-500 outline-none focus:border-indigo-500"
                      />
                    </div>
                    <div className="max-h-48 overflow-y-auto py-1">
                      <button
                        onClick={() => { setSelectedRepo(''); setRepoSearchOpen(false); setRepoSearch('') }}
                        className="w-full px-3 py-1.5 text-left text-xs text-gray-300 hover:bg-gray-700"
                      >
                        All repositories
                      </button>
                      {filteredRepos.map((r) => (
                        <button
                          key={r}
                          onClick={() => { setSelectedRepo(r); setRepoSearchOpen(false); setRepoSearch('') }}
                          className="w-full px-3 py-1.5 text-left text-xs text-gray-300 hover:bg-gray-700"
                        >
                          {r}
                        </button>
                      ))}
                    </div>
                  </div>
                </>
              )}
            </div>

            {/* Status filter (dashboard only) */}
            {activePage === 'dashboard' && (
              <select
                value={selectedStatus}
                onChange={(e) => setSelectedStatus(e.target.value)}
                className="rounded-lg border border-gray-700 bg-gray-800 px-3 py-1.5 text-xs text-gray-300 outline-none hover:border-gray-600 focus:border-indigo-500 transition-colors"
              >
                {STATUS_OPTIONS.map((opt) => (
                  <option key={opt.id} value={opt.id}>{opt.label}</option>
                ))}
              </select>
            )}
          </div>
        </header>

        {/* Page content */}
        <div className="p-6">
          {activePage === 'dashboard' && (
            <div className="space-y-6">
              <MetricsCards
                running={running}
                queued={queued}
                avgQueueTime={avgQueueTime}
                avgRunTime={avgRunTime}
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
              <WorkflowTable
                key={`${selectedRepo}:${selectedStatus}`}
                ready={ready}
                refreshSignal={workflowRefresh}
                repo={selectedRepo}
                status={selectedStatus}
              />
            </div>
          )}

          {activePage === 'failures' && (
            <FailureAnalytics ready={ready} repo={selectedRepo} />
          )}

          {activePage === 'labels' && (
            <LabelDemand ready={ready} repo={selectedRepo} />
          )}
        </div>
      </main>
    </div>
  )
}
