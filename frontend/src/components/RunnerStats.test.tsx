import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ReactElement } from 'react'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { RunnerStats } from './RunnerStats'
import type { RunnerStatsResponse } from '../api/types'

const { getRunnerStats } = vi.hoisted(() => ({ getRunnerStats: vi.fn() }))

vi.mock('../api/client', () => ({ getRunnerStats }))

// ResponsiveContainer sizes itself with ResizeObserver, which jsdom lacks;
// give the chart a fixed size instead.
vi.mock('recharts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('recharts')>()
  const { cloneElement } = await import('react')
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: ReactElement }) =>
      cloneElement(children, { width: 800, height: 300 }),
  }
})

function stats(): RunnerStatsResponse {
  return {
    summary: [
      {
        runner_name: 'runner-a',
        runner_group_name: 'Default',
        total_jobs: 5,
        succeeded: 1,
        failed: 2,
        cancelled: 1,
        other: 1,
        failure_rate: 0.4,
        avg_duration_seconds: 80,
        running: 0,
        last_job_at: new Date(Date.now() - 10 * 60 * 1000).toISOString(),
      },
      {
        runner_name: 'runner-c',
        runner_group_name: '',
        total_jobs: 0,
        succeeded: 0,
        failed: 0,
        cancelled: 0,
        other: 0,
        failure_rate: 0,
        avg_duration_seconds: 0,
        running: 1,
        last_job_at: null,
      },
    ],
  }
}

describe('RunnerStats', () => {
  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('lists per-runner statistics for the selected repository', async () => {
    getRunnerStats.mockResolvedValue(stats())

    render(<RunnerStats ready repo="owner/repo" />)

    expect(getRunnerStats).toHaveBeenCalledWith('day', 'owner/repo')
    const table = screen.getByRole('table')
    const rowA = (await within(table).findByText('runner-a')).closest('tr')!
    const cellsA = within(rowA).getAllByRole('cell').map((c) => c.textContent)
    expect(cellsA).toEqual(['runner-a', 'Default', '5', '1', '2', '1', '40.0%', '1m 20s', '0', '10m ago'])

    // A runner with only a running job has no completed-job stats.
    const rowC = within(table).getByText('runner-c').closest('tr')!
    const cellsC = within(rowC).getAllByRole('cell').map((c) => c.textContent)
    expect(cellsC).toEqual(['runner-c', '-', '0', '0', '0', '0', '-', '-', '1', '-'])

    // Only runners with completed jobs are charted.
    expect(screen.queryByText('No completed jobs on any runner in this period')).not.toBeInTheDocument()
  })

  it('shows an empty state when no runner has activity', async () => {
    getRunnerStats.mockResolvedValue({ summary: [] })

    render(<RunnerStats ready repo="" />)

    await waitFor(() => expect(getRunnerStats).toHaveBeenCalledOnce())
    expect(await screen.findByText('No runner activity for this period')).toBeInTheDocument()
    expect(screen.getByText('No completed jobs on any runner in this period')).toBeInTheDocument()
  })

  it('reloads when the period changes', async () => {
    getRunnerStats.mockResolvedValue(stats())

    render(<RunnerStats ready repo="" />)
    await within(screen.getByRole('table')).findByText('runner-a')

    fireEvent.click(screen.getByRole('button', { name: '1w' }))
    await waitFor(() => expect(getRunnerStats).toHaveBeenCalledWith('week', ''))
  })

  it('waits for the app to be ready before loading', () => {
    render(<RunnerStats ready={false} repo="" />)
    expect(getRunnerStats).not.toHaveBeenCalled()
  })
})
