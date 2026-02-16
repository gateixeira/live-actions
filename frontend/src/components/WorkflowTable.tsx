import { useState, useEffect, useCallback } from 'react'
import { Box, Text, Label, Spinner, RelativeTime, Link } from '@primer/react'
import {
  ChevronDownIcon,
  ChevronRightIcon,
  CheckCircleFillIcon,
  XCircleFillIcon,
  ClockIcon,
  SkipFillIcon,
  HourglassIcon,
} from '@primer/octicons-react'
import type { WorkflowRun, WorkflowJob, Pagination } from '../api/types'
import { getWorkflowRuns, getWorkflowJobs } from '../api/client'

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
      <Box as="td" sx={{ p: 2, pl: 5, fontSize: 0 }}>
        {job.name}
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
    setLoading(true)
    getWorkflowJobs(run.id)
      .then((r) => setJobs(r.workflow_jobs ?? []))
      .catch(() => setJobs([]))
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
        <Box as="td" sx={{ p: 2, fontSize: 1 }}>
          {run.name}
        </Box>
        <Box as="td" sx={{ p: 2, fontSize: 0, color: 'fg.muted' }}>
          {run.repository_name}
        </Box>
        <Box as="td" sx={{ p: 2, fontSize: 0 }}>
          {run.display_title}
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

export function WorkflowTable({ ready }: { ready: boolean }) {
  const [runs, setRuns] = useState<WorkflowRun[]>([])
  const [pagination, setPagination] = useState<Pagination | null>(null)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [refresh, setRefresh] = useState(0)

  const load = useCallback(() => {
    setLoading(true)
    getWorkflowRuns(page, 15)
      .then((r) => {
        setRuns(r.workflow_runs ?? [])
        setPagination(r.pagination)
      })
      .catch(() => setRuns([]))
      .finally(() => setLoading(false))
  }, [page])

  useEffect(() => {
    if (!ready) return
    load()
  }, [load, ready])

  // Expose a trigger for SSE refreshes
  const triggerRefresh = useCallback(() => {
    setRefresh((r) => r + 1)
    load()
  }, [load])

  // Store ref for external access
  ;(WorkflowTable as any)._refresh = triggerRefresh

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Text sx={{ fontSize: 1, fontWeight: 'bold' }}>Recent Workflow Runs</Text>
        {pagination && (
          <Text sx={{ fontSize: 0, color: 'fg.muted' }}>
            Page {pagination.current_page} of {pagination.total_pages} ({pagination.total_count} total)
          </Text>
        )}
      </Box>

      <Box
        sx={{
          border: '1px solid',
          borderColor: 'border.default',
          borderRadius: 2,
          overflow: 'hidden',
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
              runs.map((run) => <RunRow key={run.id} run={run} refresh={refresh} />)
            )}
          </Box>
        </Box>
      </Box>

      {pagination && pagination.total_pages > 1 && (
        <Box sx={{ display: 'flex', justifyContent: 'center', gap: 2, mt: 3 }}>
          <Box
            as="button"
            sx={{
              px: 3,
              py: 1,
              fontSize: 0,
              borderRadius: 2,
              border: '1px solid',
              borderColor: 'border.default',
              bg: 'canvas.default',
              color: 'fg.default',
              cursor: pagination.has_previous ? 'pointer' : 'not-allowed',
              opacity: pagination.has_previous ? 1 : 0.5,
              '&:hover': pagination.has_previous ? { bg: 'canvas.subtle' } : {},
            }}
            onClick={() => pagination.has_previous && setPage((p) => p - 1)}
            disabled={!pagination.has_previous}
          >
            Previous
          </Box>
          <Box
            as="button"
            sx={{
              px: 3,
              py: 1,
              fontSize: 0,
              borderRadius: 2,
              border: '1px solid',
              borderColor: 'border.default',
              bg: 'canvas.default',
              color: 'fg.default',
              cursor: pagination.has_next ? 'pointer' : 'not-allowed',
              opacity: pagination.has_next ? 1 : 0.5,
              '&:hover': pagination.has_next ? { bg: 'canvas.subtle' } : {},
            }}
            onClick={() => pagination.has_next && setPage((p) => p + 1)}
            disabled={!pagination.has_next}
          >
            Next
          </Box>
        </Box>
      )}
    </Box>
  )
}
