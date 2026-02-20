import { Box, Heading, Text } from '@primer/react'

interface Props {
  running: number
  queued: number
  avgQueueTime: number
  peakDemand: number
}

function formatSeconds(s: number): string {
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

function Card({ label, value }: { label: string; value: React.ReactNode; variant?: string }) {
  return (
    <Box
      sx={{
        p: 3,
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'border.default',
        bg: 'canvas.default',
        flex: 1,
        minWidth: 160,
      }}
    >
      <Text sx={{ fontSize: 0, color: 'fg.muted', display: 'block', mb: 1 }}>{label}</Text>
      <Heading as="h3" sx={{ fontSize: 4, fontWeight: 'bold' }}>
        {value}
      </Heading>
    </Box>
  )
}

export function MetricsCards({ running, queued, avgQueueTime, peakDemand }: Props) {
  return (
    <Box sx={{ display: 'flex', gap: 3, flexWrap: 'wrap', mb: 4 }}>
      <Card label="Running Jobs" value={running} />
      <Card label="Queued Jobs" value={queued} />
      <Card label="Avg Queue Time" value={formatSeconds(avgQueueTime)} />
      <Card label="Peak Demand" value={peakDemand} />
    </Box>
  )
}
