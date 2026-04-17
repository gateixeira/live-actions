import { clsx } from 'clsx'
import {
  CheckCircle,
  XCircle,
  Clock,
  Loader2,
  Ban,
  Hourglass,
  AlertCircle,
} from 'lucide-react'

const STATUS_MAP: Record<string, { className: string; icon: typeof Clock; text: string }> = {
  queued:           { className: 'bg-amber-400/10 text-amber-400',   icon: Hourglass,    text: 'Queued' },
  in_progress:      { className: 'bg-blue-400/10 text-blue-400',     icon: Loader2,      text: 'Running' },
  success:          { className: 'bg-emerald-400/10 text-emerald-400', icon: CheckCircle, text: 'Success' },
  failure:          { className: 'bg-red-400/10 text-red-400',       icon: XCircle,      text: 'Failed' },
  cancelled:        { className: 'bg-gray-400/10 text-gray-400',     icon: Ban,          text: 'Cancelled' },
  skipped:          { className: 'bg-gray-400/10 text-gray-400',     icon: Ban,          text: 'Skipped' },
  timed_out:        { className: 'bg-red-400/10 text-red-400',       icon: Clock,        text: 'Timed Out' },
  requested:        { className: 'bg-amber-400/10 text-amber-400',   icon: Clock,        text: 'Requested' },
  waiting:          { className: 'bg-amber-400/10 text-amber-400',   icon: Clock,        text: 'Waiting' },
  action_required:  { className: 'bg-amber-400/10 text-amber-400',   icon: AlertCircle,  text: 'Action Required' },
  stale:            { className: 'bg-gray-400/10 text-gray-400',     icon: Clock,        text: 'Stale' },
}

const FALLBACK = { className: 'bg-gray-400/10 text-gray-400', icon: Clock, text: 'Unknown' }

interface StatusBadgeProps {
  status: string
  conclusion?: string
}

export function StatusBadge({ status, conclusion }: StatusBadgeProps) {
  const effective = conclusion || status
  const s = STATUS_MAP[effective] ?? { ...FALLBACK, text: effective }
  const Icon = s.icon

  return (
    <span
      className={clsx(
        'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium',
        s.className,
      )}
    >
      <Icon className={clsx('h-3 w-3', effective === 'in_progress' && 'animate-spin')} />
      {s.text}
    </span>
  )
}
