import type {
  WorkflowRunsResponse,
  WorkflowJobsResponse,
  MetricsResponse,
  FailureAnalyticsResponse,
  LabelDemandResponse,
  RepositoriesResponse,
  Period,
} from './types'

let csrfToken: string | null = null

function getCsrfToken(): string | null {
  return csrfToken
}

// Fetch /api/csrf to get the CSRF token and set the cookie
export async function initCsrf(): Promise<void> {
  if (getCsrfToken()) return
  await refreshCsrf()
}

// Force-refresh the CSRF token (called on init and after 403 errors)
async function refreshCsrf(): Promise<void> {
  try {
    const res = await fetch('/api/csrf', { credentials: 'same-origin' })
    const data = await res.json()
    if (data.token) csrfToken = data.token
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
  if (res.status === 403) {
    // CSRF token may be stale after server restart — refresh and retry once
    await refreshCsrf()
    const retry = await fetch(url, {
      headers: headers(),
      credentials: 'same-origin',
    })
    if (!retry.ok) throw new Error(`${retry.status} ${retry.statusText}`)
    return retry.json()
  }
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

function repoParam(repo: string): string {
  return repo ? `&repo=${encodeURIComponent(repo)}` : ''
}

export async function getWorkflowRuns(
  page = 1,
  limit = 25,
  repo = '',
): Promise<WorkflowRunsResponse> {
  return fetchJson(`/api/workflow-runs?page=${page}&limit=${limit}${repoParam(repo)}`)
}

export async function getWorkflowJobs(
  runId: number,
): Promise<WorkflowJobsResponse> {
  return fetchJson(`/api/workflow-jobs/${runId}`)
}

export async function getMetrics(period: Period): Promise<MetricsResponse> {
  return fetchJson(`/api/metrics/query_range?period=${period}`)
}

export async function getFailureAnalytics(
  period: Period,
  repo = '',
): Promise<FailureAnalyticsResponse> {
  return fetchJson(`/api/analytics/failures?period=${period}${repoParam(repo)}`)
}

export async function getLabelDemand(
  period: Period,
  repo = '',
): Promise<LabelDemandResponse> {
  return fetchJson(`/api/analytics/labels?period=${period}${repoParam(repo)}`)
}

export async function getRepositories(): Promise<RepositoriesResponse> {
  return fetchJson('/api/repositories')
}
