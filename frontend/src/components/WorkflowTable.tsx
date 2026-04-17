import { useState, useEffect, useCallback } from 'react'
import { ChevronDown, ChevronRight, ExternalLink, Loader2 } from 'lucide-react'
import { StatusBadge } from './StatusBadge'
import type { WorkflowRun, WorkflowJob, Pagination as PaginationData } from '../api/types'
import { getWorkflowRuns, getWorkflowJobs } from '../api/client'

const MAX_TEXT_LEN = 50
function truncate(text: string): string {
  return text.length > MAX_TEXT_LEN ? text.slice(0, MAX_TEXT_LEN) + '…' : text
}

function duration(start?: string, end?: string, status?: string): string {
  if (!start) return '-'
  const s = new Date(start).getTime()
  const e =
    status === 'in_progress' ? Date.now() : end ? new Date(end).getTime() : null
  if (!e) return '-'
  const ms = e - s
  if (ms < 1000) return '< 1s'
  const sec = Math.floor(ms / 1000)
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  const remSec = sec % 60
  if (min < 60) return `${min}m ${remSec}s`
  const hr = Math.floor(min / 60)
  return `${hr}h ${min % 60}m`
}

function relativeTime(dateStr: string): string {
  const now = Date.now()
  const then = new Date(dateStr).getTime()
  const diffMs = now - then
  const seconds = Math.floor(diffMs / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

function JobRow({ job }: { job: WorkflowJob }) {
  return (
    <tr className="border-t border-gray-800/50 hover:bg-gray-800/30">
      <td className="py-2 pl-10 pr-3 text-sm">
        {job.html_url ? (
          <a
            href={job.html_url}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            className="text-indigo-400 hover:text-indigo-300"
          >
            {truncate(job.name)}
          </a>
        ) : (
          <span className="text-gray-300">{truncate(job.name)}</span>
        )}
      </td>
      <td className="py-2 px-3">
        <StatusBadge status={job.status} conclusion={job.conclusion} />
      </td>
      <td className="py-2 px-3">
        <div className="flex flex-wrap gap-1">
          {job.labels?.map((label) => (
            <span
              key={label}
              className="inline-block rounded-md bg-gray-800 px-2 py-0.5 text-xs text-gray-400"
            >
              {label}
            </span>
          )) || <span className="text-gray-600">-</span>}
        </div>
      </td>
      <td className="py-2 px-3 text-sm tabular-nums text-gray-400">
        {duration(job.started_at, job.completed_at, job.status)}
      </td>
    </tr>
  )
}

function RunRow({ run, refresh }: { run: WorkflowRun; refresh: number }) {
  const [expanded, setExpanded] = useState(false)
  const [jobs, setJobs] = useState<WorkflowJob[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!expanded) return
    setLoading(true) // eslint-disable-line react-hooks/set-state-in-effect
    getWorkflowJobs(run.id)
      .then((r) => setJobs(r.workflow_jobs ?? []))
      .catch((err) => { console.error('Failed to load jobs', err); setJobs([]) })
      .finally(() => setLoading(false))
  }, [expanded, run.id, refresh])

  return (
    <>
      <tr
        className="cursor-pointer border-t border-gray-800 transition-colors hover:bg-gray-800/50"
        onClick={() => setExpanded((e) => !e)}
      >
        <td className="w-8 py-3 pl-4 pr-1 text-gray-500">
          {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
        </td>
        <td className="py-3 px-3 text-sm">
          <a
            href={run.html_url}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            className="inline-flex items-center gap-1 text-indigo-400 hover:text-indigo-300"
          >
            #{run.id}
            <ExternalLink className="h-3 w-3" />
          </a>
        </td>
        <td className="py-3 px-3 text-sm text-gray-200" title={run.name}>
          {truncate(run.name)}
        </td>
        <td className="py-3 px-3 text-sm text-gray-500">
          {run.repository_name}
        </td>
        <td className="py-3 px-3 text-sm text-gray-400" title={run.display_title}>
          {truncate(run.display_title)}
        </td>
        <td className="py-3 px-3">
          <StatusBadge status={run.status} conclusion={run.conclusion} />
        </td>
        <td className="py-3 px-3 text-sm tabular-nums text-gray-400">
          {duration(run.run_started_at || run.created_at, run.updated_at, run.status)}
        </td>
        <td className="py-3 px-3 text-sm text-gray-500">
          {relativeTime(run.created_at)}
        </td>
      </tr>
      {expanded && (
        <tr>
          <td colSpan={8} className="p-0">
            {loading ? (
              <div className="flex items-center justify-center py-6">
                <Loader2 className="h-5 w-5 animate-spin text-gray-500" />
              </div>
            ) : jobs.length === 0 ? (
              <div className="py-4 text-center text-sm text-gray-600">
                No jobs found
              </div>
            ) : (
              <table className="w-full">
                <thead>
                  <tr className="text-xs text-gray-500">
                    <th className="py-2 pl-10 pr-3 text-left font-medium">Job</th>
                    <th className="py-2 px-3 text-left font-medium">Status</th>
                    <th className="py-2 px-3 text-left font-medium">Runner</th>
                    <th className="py-2 px-3 text-left font-medium">Duration</th>
                  </tr>
                </thead>
                <tbody>
                  {jobs.map((j) => (
                    <JobRow key={j.id} job={j} />
                  ))}
                </tbody>
              </table>
            )}
          </td>
        </tr>
      )}
    </>
  )
}

export function WorkflowTable({ ready, refreshSignal, repo, status }: { ready: boolean; refreshSignal: number; repo: string; status: string }) {
  const [runs, setRuns] = useState<WorkflowRun[]>([])
  const [pagination, setPagination] = useState<PaginationData | null>(null)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)

  const load = useCallback(() => {
    setLoading(true)
    getWorkflowRuns(page, 15, repo, status)
      .then((r) => {
        setRuns(r.workflow_runs ?? [])
        setPagination(r.pagination)
      })
      .catch((err) => { console.error('Failed to load workflow runs', err); setRuns([]) })
      .finally(() => setLoading(false))
  }, [page, repo, status])

  useEffect(() => {
    if (!ready) return
    load() // eslint-disable-line react-hooks/set-state-in-effect
  }, [load, ready, refreshSignal])

  const totalPages = pagination?.total_pages ?? 1

  return (
    <div>
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-200">
          Recent Workflow Runs
          {pagination && (
            <span className="ml-2 text-xs font-normal text-gray-500">
              {pagination.total_count} total
            </span>
          )}
        </h3>
        {totalPages > 1 && (
          <div className="flex items-center gap-1">
            <button
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
              className="rounded-md px-2.5 py-1 text-xs text-gray-400 hover:bg-gray-800 disabled:opacity-30 disabled:cursor-not-allowed"
            >
              Previous
            </button>
            <span className="px-2 text-xs text-gray-500">
              {page} / {totalPages}
            </span>
            <button
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
              className="rounded-md px-2.5 py-1 text-xs text-gray-400 hover:bg-gray-800 disabled:opacity-30 disabled:cursor-not-allowed"
            >
              Next
            </button>
          </div>
        )}
      </div>

      <div className="overflow-hidden rounded-xl border border-gray-800 bg-gray-900">
        <table className="w-full text-left">
          <thead>
            <tr className="border-b border-gray-800 bg-gray-800/40 text-xs font-medium text-gray-500 uppercase tracking-wider">
              <th className="w-8 py-3 pl-4 pr-1" />
              <th className="py-3 px-3">Build</th>
              <th className="py-3 px-3">Workflow</th>
              <th className="py-3 px-3">Repository</th>
              <th className="py-3 px-3">Title</th>
              <th className="py-3 px-3">Status</th>
              <th className="py-3 px-3">Duration</th>
              <th className="py-3 px-3">Created</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={8} className="py-12 text-center">
                  <Loader2 className="mx-auto h-6 w-6 animate-spin text-gray-500" />
                </td>
              </tr>
            ) : runs.length === 0 ? (
              <tr>
                <td colSpan={8} className="py-12 text-center text-sm text-gray-600">
                  No workflow runs found
                </td>
              </tr>
            ) : (
              runs.map((run) => <RunRow key={run.id} run={run} refresh={refreshSignal} />)
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
