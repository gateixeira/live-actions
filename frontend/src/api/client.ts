import type {
  WorkflowRunsResponse,
  WorkflowJobsResponse,
  MetricsResponse,
  Period,
} from './types'

let csrfToken: string | null = null

function getCsrfToken(): string | null {
  if (csrfToken) return csrfToken
  // Read from meta tag injected by the Go server (production)
  const meta = document.querySelector('meta[name="csrf-token"]')
  csrfToken = meta?.getAttribute('content') ?? null
  return csrfToken
}

// Fetch /dashboard to set the CSRF cookie and extract the token from the response
export async function initCsrf(): Promise<void> {
  if (getCsrfToken()) return
  try {
    const res = await fetch('/dashboard', { credentials: 'same-origin' })
    const html = await res.text()
    const match = html.match(/meta name="csrf-token" content="([^"]+)"/)
    if (match) csrfToken = match[1]
  } catch {
    // Non-fatal: API calls will fail with 403 if CSRF is missing
  }
}

function headers(): Record<string, string> {
  const csrf = getCsrfToken()
  const h: Record<string, string> = { 'Content-Type': 'application/json' }
  if (csrf) h['X-CSRF-Token'] = csrf
  return h
}

async function fetchJson<T>(url: string): Promise<T> {
  const res = await fetch(url, {
    headers: headers(),
    credentials: 'same-origin',
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export async function getWorkflowRuns(
  page = 1,
  limit = 25,
): Promise<WorkflowRunsResponse> {
  return fetchJson(`/api/workflow-runs?page=${page}&limit=${limit}`)
}

export async function getWorkflowJobs(
  runId: number,
): Promise<WorkflowJobsResponse> {
  return fetchJson(`/api/workflow-jobs/${runId}`)
}

function periodParams(period: Period) {
  const now = Math.floor(Date.now() / 1000)
  const ranges: Record<Period, { seconds: number; step: string }> = {
    hour: { seconds: 3600, step: '15s' },
    day: { seconds: 86400, step: '5m' },
    week: { seconds: 604800, step: '30m' },
    month: { seconds: 2592000, step: '2h' },
  }
  const r = ranges[period]
  return `period=${period}&start=${now - r.seconds}&end=${now}&step=${r.step}`
}

export async function getMetrics(period: Period): Promise<MetricsResponse> {
  return fetchJson(`/api/metrics/query_range?${periodParams(period)}`)
}
