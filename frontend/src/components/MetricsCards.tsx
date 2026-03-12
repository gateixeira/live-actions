import { Box } from '@primer/react'
import { Card } from './Card'
import { formatSeconds } from '../utils/format'

interface Props {
  running: number
  queued: number
  avgQueueTime: number
  avgRunTime: number
  peakDemand: number
}

export function MetricsCards({ running, queued, avgQueueTime, avgRunTime, peakDemand }: Props) {
  return (
    <Box sx={{ display: 'flex', gap: 3, flexWrap: 'wrap', mb: 4 }}>
      <Card label="Running Jobs" value={running} />
      <Card label="Queued Jobs" value={queued} />
      <Card label="Avg Queue Time" value={formatSeconds(avgQueueTime)} />
      <Card label="Avg Run Time" value={formatSeconds(avgRunTime)} />
      <Card label="Peak Demand" value={peakDemand} />
    </Box>
  )
}
