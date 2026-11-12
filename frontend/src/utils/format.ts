import type { Period } from '../api/types'

export const PERIODS: { label: string; value: Period }[] = [
  { label: '1h', value: 'hour' },
  { label: '1d', value: 'day' },
  { label: '1w', value: 'week' },
  { label: '1m', value: 'month' },
]

export function formatTime(ts: number, period: Period): string {
  const d = new Date(ts * 1000)
  if (period === 'hour' || period === 'day')
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

export function formatSeconds(s: number): string {
  if (!s || s === 0) return '0s'
  const total = Math.round(s)
  if (total < 60) return `${total}s`
  const m = Math.floor(total / 60)
  const rem = total % 60
  if (m < 60) return rem > 0 ? `${m}m ${rem}s` : `${m}m`
  const h = Math.floor(m / 60)
  const rm = m % 60
  return rm > 0 ? `${h}h ${rm}m` : `${h}h`
}

export function relativeTime(dateStr: string): string {
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
