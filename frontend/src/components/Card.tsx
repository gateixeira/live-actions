interface CardProps {
  label: string
  value: React.ReactNode
  sub?: string
  accent?: 'emerald' | 'red' | 'amber' | 'blue' | 'default'
}

const ACCENT_CLASSES: Record<string, string> = {
  emerald: 'text-emerald-400',
  red: 'text-red-400',
  amber: 'text-amber-400',
  blue: 'text-blue-400',
  default: 'text-white',
}

export function Card({ label, value, sub, accent = 'default' }: CardProps) {
  return (
    <div className="flex min-w-[160px] flex-1 flex-col rounded-xl border border-gray-800 bg-gray-900 p-4">
      <span className="text-xs font-medium text-gray-500 uppercase tracking-wider">
        {label}
      </span>
      <span className={`mt-1 text-2xl font-semibold tabular-nums ${ACCENT_CLASSES[accent]}`}>
        {value}
      </span>
      {sub && (
        <span className="mt-1 text-xs text-gray-500">{sub}</span>
      )}
    </div>
  )
}
