import { Card } from './Card'
import { formatSeconds } from '../utils/format'
import { Activity, Clock, Timer, TrendingUp, Layers } from 'lucide-react'

interface Props {
  running: number
  queued: number
  avgQueueTime: number
  avgRunTime: number
  peakDemand: number
}

export function MetricsCards({ running, queued, avgQueueTime, avgRunTime, peakDemand }: Props) {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5 mb-6">
      <MetricCard icon={Activity} label="Running" value={running} accent="emerald" />
      <MetricCard icon={Layers} label="Queued" value={queued} accent={queued > 0 ? 'amber' : 'default'} />
      <MetricCard icon={Clock} label="Avg Queue" value={formatSeconds(avgQueueTime)} />
      <MetricCard icon={Timer} label="Avg Runtime" value={formatSeconds(avgRunTime)} />
      <MetricCard icon={TrendingUp} label="Peak Demand" value={peakDemand} />
    </div>
  )
}

function MetricCard({ icon: Icon, label, value, accent = 'default' }: {
  icon: typeof Activity
  label: string
  value: React.ReactNode
  accent?: 'emerald' | 'red' | 'amber' | 'blue' | 'default'
}) {
  return (
    <Card
      label={label}
      value={
        <span className="flex items-center gap-2">
          <Icon className="h-5 w-5 text-gray-600" />
          {value}
        </span>
      }
      accent={accent}
    />
  )
}
