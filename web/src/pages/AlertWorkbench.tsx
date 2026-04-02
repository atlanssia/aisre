import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, Clock, Server, ChevronRight, RefreshCw } from 'lucide-react'
import { incidents } from '@/api/client'
import { SeverityBadge } from '@/components/rca/SeverityBadge'
import { StatusBadge } from '@/components/rca/StatusBadge'
import { cn } from '@/lib/utils'
import type { Incident } from '@/types'

export function AlertWorkbench() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['incidents'],
    queryFn: () => incidents.list({ limit: 50 }),
    refetchInterval: 15_000,
  })

  const items = data ?? []

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <header className="px-6 py-4 border-b border-border flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold text-foreground flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-amber-400" />
            Alert Workbench
          </h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Active incidents requiring attention
          </p>
        </div>
        <button
          onClick={() => refetch()}
          className="flex items-center gap-2 px-3 py-1.5 text-sm rounded-md bg-secondary text-secondary-foreground hover:bg-secondary/80 transition-colors"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          Refresh
        </button>
      </header>

      {/* Stats */}
      <div className="px-6 py-3 border-b border-border grid grid-cols-4 gap-4">
        <StatCard
          label="Critical"
          count={items.filter((i) => i.severity === 'critical').length}
          color="text-red-400"
        />
        <StatCard
          label="High"
          count={items.filter((i) => i.severity === 'high').length}
          color="text-orange-400"
        />
        <StatCard
          label="Open"
          count={items.filter((i) => i.status === 'open').length}
          color="text-blue-400"
        />
        <StatCard
          label="Analyzing"
          count={items.filter((i) => i.status === 'analyzing').length}
          color="text-amber-400"
        />
      </div>

      {/* Incident Table */}
      <div className="flex-1 overflow-auto px-6 py-3">
        {isLoading ? (
          <div className="flex items-center justify-center h-64 text-muted-foreground">
            Loading incidents...
          </div>
        ) : error ? (
          <div className="flex items-center justify-center h-64 text-destructive">
            Failed to load incidents: {error.message}
          </div>
        ) : items.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64 text-muted-foreground">
            <AlertTriangle className="h-8 w-8 mb-2 opacity-50" />
            <p>No active incidents. You can go back to sleep.</p>
          </div>
        ) : (
          <div className="space-y-1">
            {items.map((incident) => (
              <IncidentRow key={incident.id} incident={incident} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function StatCard({ label, count, color }: { label: string; count: number; color: string }) {
  return (
    <div className="bg-card rounded-lg border border-border px-4 py-2.5">
      <div className={cn('text-2xl font-bold tabular-nums', color)}>{count}</div>
      <div className="text-xs text-muted-foreground mt-0.5">{label}</div>
    </div>
  )
}

function IncidentRow({ incident }: { incident: Incident }) {
  const timeAgo = formatTimeAgo(incident.created_at)

  return (
    <div className="flex items-center gap-4 px-4 py-3 rounded-lg hover:bg-secondary/50 transition-colors cursor-pointer group">
      <SeverityBadge severity={incident.severity} />
      <StatusBadge status={incident.status} />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 text-sm text-foreground">
          <Server className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
          <span className="font-medium truncate">{incident.service_name}</span>
          {incident.source && (
            <span className="text-xs text-muted-foreground">via {incident.source}</span>
          )}
        </div>
        {incident.trace_id && (
          <div className="text-xs text-muted-foreground mt-0.5 font-mono">
            trace: {incident.trace_id}
          </div>
        )}
      </div>
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <Clock className="h-3 w-3" />
        {timeAgo}
      </div>
      <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
    </div>
  )
}

function formatTimeAgo(isoDate: string): string {
  const diff = Date.now() - new Date(isoDate).getTime()
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}
