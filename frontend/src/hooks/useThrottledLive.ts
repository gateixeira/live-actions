import { useCallback, useEffect, useRef, useState } from 'react'

interface Options {
  /** Commit interval in milliseconds. Defaults to 30s. */
  intervalMs?: number
  /** When true, commits are deferred until it becomes false again. */
  paused?: boolean
}

/**
 * useThrottledLive buffers high-frequency updates in a ref and only commits
 * them to React state every `intervalMs`. While `paused` is true, commits are
 * deferred so the UI stays stable while the user is interacting with the page.
 * When `paused` flips back to false, any buffered value is committed
 * immediately.
 *
 * Calling the returned setter does NOT trigger a re-render — it only updates
 * the buffered value. Re-renders happen on the timer tick (when the buffered
 * value differs from the committed one) or on resume.
 *
 * Returns a tuple of `[committedValue, setLatest]`.
 */
export function useThrottledLive<T>(
  initial: T,
  opts: Options = {},
): [T, (v: T) => void] {
  const { intervalMs = 30_000, paused = false } = opts
  const [committed, setCommitted] = useState<T>(initial)
  const latestRef = useRef<T>(initial)
  const pausedRef = useRef(paused)

  useEffect(() => {
    pausedRef.current = paused
    if (!paused) {
      // On resume, commit immediately if the buffered value differs.
      setCommitted((prev) =>
        Object.is(latestRef.current, prev) ? prev : latestRef.current,
      )
    }
  }, [paused])

  useEffect(() => {
    const id = setInterval(() => {
      if (pausedRef.current) return
      setCommitted((prev) =>
        Object.is(latestRef.current, prev) ? prev : latestRef.current,
      )
    }, intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])

  const setLatest = useCallback((v: T) => {
    latestRef.current = v
  }, [])

  return [committed, setLatest]
}

