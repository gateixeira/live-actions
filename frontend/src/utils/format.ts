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
