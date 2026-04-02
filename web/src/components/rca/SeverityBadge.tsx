import { cn } from '@/lib/utils'
import type { Severity } from '@/types'

const severityConfig: Record<Severity, { label: string; className: string }> = {
  critical: {
    label: 'CRITICAL',
    className: 'bg-red-500/15 text-red-400 border-red-500/30',
  },
  high: {
    label: 'HIGH',
    className: 'bg-orange-500/15 text-orange-400 border-orange-500/30',
  },
  medium: {
    label: 'MEDIUM',
    className: 'bg-yellow-500/15 text-yellow-400 border-yellow-500/30',
  },
  low: {
    label: 'LOW',
    className: 'bg-blue-500/15 text-blue-400 border-blue-500/30',
  },
  info: {
    label: 'INFO',
    className: 'bg-slate-500/15 text-slate-400 border-slate-500/30',
  },
}

interface SeverityBadgeProps {
  severity: Severity
  className?: string
}

export function SeverityBadge({ severity, className }: SeverityBadgeProps) {
  const config = severityConfig[severity] || severityConfig.info
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
