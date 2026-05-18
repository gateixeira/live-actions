import { useCallback, useEffect, useRef, useState } from 'react'

interface Options {
  /**
   * When true, writes are buffered and not committed to React state until
   * `paused` flips back to false. While unpaused, writes are committed
   * synchronously (no throttling).
   */
  paused?: boolean
}

/**
 * usePausableLive exposes a `[committed, setLatest]` pair where `setLatest`
 * commits to React state immediately, EXCEPT while `paused` is true. While
 * paused, the latest value is buffered in a ref; it is committed when `paused`
 * flips back to false.
 *
 * This is used for high-frequency SSE-driven counters that should feel "live"
 * but must not cause flicker while the user is actively interacting with the
 * page (hovering rows, focusing inputs, etc.).
 */
export function usePausableLive<T>(
  initial: T,
  opts: Options = {},
): [T, (v: T) => void] {
  const { paused = false } = opts
  const [committed, setCommitted] = useState<T>(initial)
  const latestRef = useRef<T>(initial)
  const hasBufferedRef = useRef(false)
  const pausedRef = useRef(paused)

  useEffect(() => {
    pausedRef.current = paused
    if (!paused && hasBufferedRef.current) {
      hasBufferedRef.current = false
      setCommitted((prev) =>
        Object.is(latestRef.current, prev) ? prev : latestRef.current,
      )
    }
  }, [paused])

  const setLatest = useCallback((v: T) => {
    latestRef.current = v
    if (pausedRef.current) {
      hasBufferedRef.current = true
      return
    }
    setCommitted((prev) => (Object.is(v, prev) ? prev : v))
  }, [])

  return [committed, setLatest]
}
