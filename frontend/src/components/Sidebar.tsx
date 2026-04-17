import { clsx } from 'clsx'
import {
  LayoutDashboard,
  AlertTriangle,
  Tags,
  Zap,
} from 'lucide-react'

type Page = 'dashboard' | 'failures' | 'labels'

interface SidebarProps {
  activePage: Page
  onNavigate: (page: Page) => void
  connected: boolean
}

const NAV_ITEMS: { id: Page; label: string; icon: typeof LayoutDashboard }[] = [
  { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { id: 'failures', label: 'Failures', icon: AlertTriangle },
  { id: 'labels', label: 'Runner Labels', icon: Tags },
]

export function Sidebar({ activePage, onNavigate, connected }: SidebarProps) {
  return (
    <aside className="fixed inset-y-0 left-0 z-30 flex w-56 flex-col border-r border-gray-800 bg-gray-900">
      {/* Logo */}
      <div className="flex h-14 items-center gap-2.5 border-b border-gray-800 px-5">
        <Zap className="h-5 w-5 text-indigo-400" />
        <span className="text-base font-semibold tracking-tight text-white">
          Live Actions
        </span>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 px-3 py-4">
        {NAV_ITEMS.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => onNavigate(id)}
            className={clsx(
              'flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
              activePage === id
                ? 'bg-indigo-500/10 text-indigo-400'
                : 'text-gray-400 hover:bg-gray-800 hover:text-gray-200',
            )}
          >
            <Icon className="h-4 w-4" />
            {label}
          </button>
        ))}
      </nav>

      {/* Connection status */}
      <div className="border-t border-gray-800 px-5 py-3">
        <div className="flex items-center gap-2 text-xs">
          <span
            className={clsx(
              'h-2 w-2 rounded-full',
              connected ? 'bg-emerald-400' : 'bg-red-400',
            )}
          />
          <span className="text-gray-500">
            {connected ? 'Connected' : 'Disconnected'}
          </span>
        </div>
      </div>
    </aside>
  )
}
