import { useState, useEffect, useCallback } from 'react'
import { Box, Text, Label, Spinner, RelativeTime, Link, SegmentedControl, IconButton } from '@primer/react'
import {
  ChevronDownIcon,
  ChevronRightIcon,
  CheckCircleFillIcon,
  XCircleFillIcon,
  ClockIcon,
  SkipFillIcon,
  HourglassIcon,
  AlertIcon,
} from '@primer/octicons-react'
import type { WorkflowRun, WorkflowJob, Pagination as PaginationData } from '../api/types'
import { getWorkflowRuns, getWorkflowJobs } from '../api/client'

const MAX_TEXT_LEN = 50
function truncate(text: string): string {
  return text.length > MAX_TEXT_LEN ? text.slice(0, MAX_TEXT_LEN) + '…' : text
}

function StatusBadge({ status, conclusion }: { status: string; conclusion?: string }) {
  const effective = conclusion || status
  const map: Record<string, { color: string; icon: React.ReactNode; text: string }> = {
    queued: { color: 'attention.fg', icon: <HourglassIcon size={14} />, text: 'Queued' },
    in_progress: { color: 'accent.fg', icon: <Spinner size="small" />, text: 'Running' },
    success: { color: 'success.fg', icon: <CheckCircleFillIcon size={14} />, text: 'Success' },
    failure: { color: 'danger.fg', icon: <XCircleFillIcon size={14} />, text: 'Failed' },
    cancelled: { color: 'fg.muted', icon: <SkipFillIcon size={14} />, text: 'Cancelled' },
    skipped: { color: 'fg.muted', icon: <SkipFillIcon size={14} />, text: 'Skipped' },
    requested: { color: 'attention.fg', icon: <ClockIcon size={14} />, text: 'Requested' },
    waiting: { color: 'attention.fg', icon: <ClockIcon size={14} />, text: 'Waiting' },
    action_required: { color: 'attention.fg', icon: <AlertIcon size={14} />, text: 'Action Required' },
  }
  const s = map[effective] ?? { color: 'fg.muted', icon: <ClockIcon size={14} />, text: effective }

  return (
    <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 1, color: s.color }}>
      {s.icon}
      <Text sx={{ fontSize: 0 }}>{s.text}</Text>
    </Box>
  )
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

function JobRow({ job }: { job: WorkflowJob }) {
  return (
    <Box
      as="tr"
      sx={{
        '&:hover': { bg: 'canvas.subtle' },
      }}
    >
      <Box as="td" sx={{ p: 2, pl: 5, fontSize: 0 }} title={job.name}>
        {job.html_url ? (
          <Link href={job.html_url} target="_blank" onClick={(e: React.MouseEvent) => e.stopPropagation()}>
            {truncate(job.name)}
          </Link>
        ) : truncate(job.name)}
      </Box>
      <Box as="td" sx={{ p: 2, fontSize: 0 }}>
        <StatusBadge status={job.status} conclusion={job.conclusion} />
      </Box>
      <Box as="td" sx={{ p: 2, fontSize: 0 }}>
        {job.labels?.map((label) => (
          <Label key={label} sx={{ mr: 1 }}>{label}</Label>
        )) || '-'}
      </Box>
      <Box as="td" sx={{ p: 2, fontSize: 0 }}>
        {duration(job.started_at, job.completed_at, job.status)}
      </Box>
    </Box>
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
      <Box
        as="tr"
        sx={{ cursor: 'pointer', '&:hover': { bg: 'canvas.subtle' } }}
        onClick={() => setExpanded((e) => !e)}
      >
        <Box as="td" sx={{ p: 2, width: 24 }}>
          {expanded ? <ChevronDownIcon size={16} /> : <ChevronRightIcon size={16} />}
        </Box>
        <Box as="td" sx={{ p: 2, fontSize: 1 }}>
          <Link href={run.html_url} target="_blank" onClick={(e: React.MouseEvent) => e.stopPropagation()}>
            {run.id}
          </Link>
        </Box>
        <Box as="td" sx={{ p: 2, fontSize: 1 }} title={run.name}>
          {truncate(run.name)}
        </Box>
        <Box as="td" sx={{ p: 2, fontSize: 0, color: 'fg.muted' }}>
          {run.repository_name}
        </Box>
        <Box as="td" sx={{ p: 2, fontSize: 0 }} title={run.display_title}>
          {truncate(run.display_title)}
        </Box>
        <Box as="td" sx={{ p: 2, fontSize: 0 }}>
          <StatusBadge status={run.status} conclusion={run.conclusion} />
        </Box>
        <Box as="td" sx={{ p: 2, fontSize: 0 }}>
          {duration(run.run_started_at || run.created_at, run.updated_at, run.status)}
        </Box>
        <Box as="td" sx={{ p: 2, fontSize: 0, color: 'fg.muted' }}>
          <RelativeTime date={new Date(run.created_at)} />
        </Box>
      </Box>
      {expanded && (
        <Box as="tr">
          <Box as="td" colSpan={8} sx={{ p: 0 }}>
            {loading ? (
              <Box sx={{ p: 3, display: 'flex', justifyContent: 'center' }}>
                <Spinner size="small" />
              </Box>
            ) : jobs.length === 0 ? (
              <Box sx={{ p: 3 }}>
                <Text sx={{ color: 'fg.muted', fontSize: 0 }}>No jobs found</Text>
              </Box>
            ) : (
              <Box
                as="table"
                sx={{
                  width: '100%',
                  borderCollapse: 'collapse',
                  bg: 'canvas.inset',
                }}
              >
                <Box as="thead">
                  <Box as="tr">
                    <Box as="th" sx={{ p: 2, pl: 5, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>
                      Job
                    </Box>
                    <Box as="th" sx={{ p: 2, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>
                      Status
                    </Box>
                    <Box as="th" sx={{ p: 2, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>
                      Runner
                    </Box>
                    <Box as="th" sx={{ p: 2, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>
                      Duration
                    </Box>
                  </Box>
                </Box>
                <Box as="tbody">
                  {jobs.map((j) => (
                    <JobRow key={j.id} job={j} />
                  ))}
                </Box>
              </Box>
            )}
          </Box>
        </Box>
      )}
    </>
  )
}

function pageRange(current: number, total: number): (number | '...')[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  const pages: (number | '...')[] = [1]
  const start = Math.max(2, current - 1)
  const end = Math.min(total - 1, current + 1)
  if (start > 2) pages.push('...')
  for (let i = start; i <= end; i++) pages.push(i)
  if (end < total - 1) pages.push('...')
  pages.push(total)
  return pages
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

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Text sx={{ fontSize: 1, fontWeight: 'bold' }}>
          Recent Workflow Runs{pagination ? ` (${pagination.total_count} runs)` : ''}
        </Text>
        {pagination && pagination.total_pages > 1 && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <IconButton aria-label="First page" icon={() => <span>«</span>} size="small" variant="invisible" disabled={page === 1} onClick={() => setPage(1)} />
            <SegmentedControl aria-label="Page" size="small" onChange={(i) => {
              const pages = pageRange(page, pagination.total_pages)
              const selected = pages[i]
              if (typeof selected === 'number') setPage(selected)
            }}>
              {pageRange(page, pagination.total_pages).map((p, i) => (
                <SegmentedControl.Button key={`${p}-${i}`} selected={p === page} disabled={p === '...'}>
                  {p === '...' ? '…' : String(p)}
                </SegmentedControl.Button>
              ))}
            </SegmentedControl>
            <IconButton aria-label="Last page" icon={() => <span>»</span>} size="small" variant="invisible" disabled={page === pagination.total_pages} onClick={() => setPage(pagination.total_pages)} />
          </Box>
        )}
      </Box>

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
          sx={{ width: '100%', borderCollapse: 'collapse' }}
        >
          <Box as="thead" sx={{ bg: 'canvas.subtle' }}>
            <Box as="tr">
              <Box as="th" sx={{ p: 2, width: 24 }} />
              <Box as="th" sx={{ p: 2, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>Build</Box>
              <Box as="th" sx={{ p: 2, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>Name</Box>
              <Box as="th" sx={{ p: 2, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>Repository</Box>
              <Box as="th" sx={{ p: 2, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>Title</Box>
              <Box as="th" sx={{ p: 2, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>Status</Box>
              <Box as="th" sx={{ p: 2, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>Duration</Box>
              <Box as="th" sx={{ p: 2, textAlign: 'left', fontSize: 0, color: 'fg.muted' }}>Created</Box>
            </Box>
          </Box>
          <Box as="tbody">
            {loading ? (
              <Box as="tr">
                <Box as="td" colSpan={8} sx={{ p: 4, textAlign: 'center' }}>
                  <Spinner size="medium" />
                </Box>
              </Box>
            ) : runs.length === 0 ? (
              <Box as="tr">
                <Box as="td" colSpan={8} sx={{ p: 4, textAlign: 'center' }}>
                  <Text sx={{ color: 'fg.muted' }}>No workflow runs found</Text>
                </Box>
              </Box>
            ) : (
              runs.map((run) => <RunRow key={run.id} run={run} refresh={refreshSignal} />)
            )}
          </Box>
        </Box>
      </Box>
    </Box>
  )
}
