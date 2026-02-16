import type {
  WorkflowRunsResponse,
  WorkflowJobsResponse,
  MetricsResponse,
  Period,
} from './types'

let csrfToken: string | null = null

function getCsrfToken(): string | null {
  return csrfToken
}

// Fetch /api/csrf to get the CSRF token and set the cookie
export async function initCsrf(): Promise<void> {
  if (getCsrfToken()) return
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

export async function getMetrics(period: Period): Promise<MetricsResponse> {
  return fetchJson(`/api/metrics/query_range?period=${period}`)
}
