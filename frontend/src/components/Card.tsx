import { Box, Heading, Text } from '@primer/react'

interface CardProps {
  label: string
  value: React.ReactNode
  color?: string
  sub?: string
}

export function Card({ label, value, color, sub }: CardProps) {
  return (
    <Box
      sx={{
        p: 3,
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'border.default',
        bg: 'canvas.default',
        flex: '1 1 160px',
        minWidth: 160,
      }}
    >
      <Text sx={{ fontSize: 0, color: 'fg.muted', display: 'block', mb: 1 }}>{label}</Text>
      <Heading as="h3" sx={{ fontSize: 4, fontWeight: 'bold', color: color || 'fg.default' }}>
        {value}
      </Heading>
      {sub && (
        <Text sx={{ fontSize: 0, color: 'fg.muted', display: 'block', mt: 1 }}>{sub}</Text>
      )}
    </Box>
  )
}
