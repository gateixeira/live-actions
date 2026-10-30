import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import App from './App'

const { getRepositories, getMetrics, initCsrf } = vi.hoisted(() => ({
  getRepositories: vi.fn(),
  getMetrics: vi.fn(),
  initCsrf: vi.fn(),
}))

vi.mock('./api/client', () => ({ getRepositories, getMetrics, initCsrf }))
vi.mock('./hooks/useSSE', () => ({ useSSE: () => ({ connected: false }) }))
vi.mock('./hooks/useThrottledLive', () => ({ useThrottledLive: () => [0, vi.fn()] }))
vi.mock('./hooks/usePausableLive', () => ({ usePausableLive: () => [null, vi.fn()] }))
vi.mock('./components/Sidebar', () => ({ Sidebar: () => null }))
vi.mock('./components/MetricsCards', () => ({ MetricsCards: () => null }))
vi.mock('./components/DemandChart', () => ({ DemandChart: () => null }))
vi.mock('./components/WorkflowTable', () => ({ WorkflowTable: () => null }))
vi.mock('./components/FailureAnalytics', () => ({ FailureAnalytics: () => null }))
vi.mock('./components/LabelDemand', () => ({ LabelDemand: () => null }))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('repository picker', () => {
  beforeEach(() => {
    initCsrf.mockResolvedValue(undefined)
    getMetrics.mockResolvedValue({})
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
    vi.clearAllMocks()
  })

  it('shows loading and empty states while the repository list is fetched', async () => {
    const request = deferred<{ repositories: string[] }>()
    getRepositories.mockReturnValueOnce(request.promise)

    render(<App />)
    await waitFor(() => expect(getRepositories).toHaveBeenCalledOnce())
    fireEvent.click(screen.getByRole('button', { name: 'All repositories' }))

    expect(screen.getByRole('status')).toHaveTextContent('Loading repositories')

    request.resolve({ repositories: [] })
    expect(await screen.findByText('No repositories available.')).toBeInTheDocument()
  })

  it('shows an error and retries a failed repository request', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    getRepositories
      .mockRejectedValueOnce(new Error('temporary failure'))
      .mockResolvedValueOnce({ repositories: ['owner/project'] })

    render(<App />)
    await waitFor(() => expect(getRepositories).toHaveBeenCalledOnce())
    fireEvent.click(screen.getByRole('button', { name: 'All repositories' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load repositories.')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByRole('button', { name: 'owner/project' })).toBeInTheDocument()
  })

  it('filters repository names case-insensitively and explains when there are no matches', async () => {
    getRepositories.mockResolvedValueOnce({
      repositories: ['owner/Autopilot', 'owner/another-project'],
    })

    render(<App />)
    await waitFor(() => expect(getRepositories).toHaveBeenCalledOnce())
    fireEvent.click(screen.getByRole('button', { name: 'All repositories' }))
    await screen.findByRole('button', { name: 'owner/Autopilot' })

    const search = screen.getByRole('textbox', { name: 'Search repositories' })
    fireEvent.change(search, { target: { value: 'AUTO' } })
    expect(screen.getByRole('button', { name: 'owner/Autopilot' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'owner/another-project' })).not.toBeInTheDocument()

    fireEvent.change(search, { target: { value: 'missing' } })
    expect(screen.getByText('No repositories match “missing”.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear search' }))
    expect(screen.getByRole('button', { name: 'owner/Autopilot' })).toBeInTheDocument()
  })
})
