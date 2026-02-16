import { useEffect, useRef, useState } from 'react'
import type { MetricsUpdateEvent, WorkflowUpdateEvent } from '../api/types'

interface SSECallbacks {
  onMetricsUpdate?: (data: MetricsUpdateEvent) => void
  onWorkflowUpdate?: (data: WorkflowUpdateEvent) => void
}

export function useSSE(callbacks: SSECallbacks) {
  const cbRef = useRef(callbacks)
  cbRef.current = callbacks
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    const es = new EventSource('/events')

    es.onopen = () => setConnected(true)

    es.addEventListener('message', (e) => {
      try {
        const outer = JSON.parse(e.data)
        // SSE can send either a raw object or a {type, data} wrapper
        if (typeof outer === 'object' && outer.type && outer.data) {
          const { type, data } = outer
          if (type === 'metrics_update') cbRef.current.onMetricsUpdate?.(data)
          if (type === 'workflow_update') cbRef.current.onWorkflowUpdate?.(data)
        }
      } catch {
        // ignore unparseable messages (e.g. initial "connected" string)
      }
    })

    es.onerror = () => setConnected(false)

    return () => {
      es.close()
      setConnected(false)
    }
  }, [])

  return { connected }
}
