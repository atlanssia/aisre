import { cn } from '@/lib/utils'
import type { IncidentStatus } from '@/types'

const statusConfig: Record<IncidentStatus, { label: string; className: string }> = {
  open: {
    label: 'OPEN',
    className: 'bg-blue-500/15 text-blue-400 border-blue-500/30',
  },
  analyzing: {
    label: 'ANALYZING',
    className: 'bg-amber-500/15 text-amber-400 border-amber-500/30 animate-pulse',
  },
  resolved: {
    label: 'RESOLVED',
    className: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
  },
  closed: {
    label: 'CLOSED',
    className: 'bg-slate-500/15 text-slate-400 border-slate-500/30',
  },
}

interface StatusBadgeProps {
  status: IncidentStatus
  className?: string
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const config = statusConfig[status] || statusConfig.open
  return (
    <span
      className={cn(
        'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border',
        config.className,
        className,
      )}
    >
      {config.label}
    </span>
  )
}
