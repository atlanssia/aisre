import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { reports, feedback } from '@/api/client'
import { ArrowLeft, CheckCircle2, XCircle, AlertTriangle, ExternalLink } from 'lucide-react'
import { Link } from 'react-router-dom'
import type { FeedbackRequest } from '@/types'

export function RCAReport() {
  const { id } = useParams<{ id: string }>()
  const reportId = Number(id)

  const { data: report, isLoading, error } = useQuery({
    queryKey: ['report', reportId],
    queryFn: () => reports.get(reportId),
    enabled: !!reportId,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        Loading RCA report...
      </div>
    )
  }

  if (error || !report) {
    return (
      <div className="flex items-center justify-center h-full text-destructive">
        Failed to load report: {error?.message}
      </div>
    )
  }

  const confidenceColor =
    report.confidence >= 0.8
      ? 'text-emerald-400'
      : report.confidence >= 0.5
        ? 'text-amber-400'
        : 'text-red-400'

  return (
    <div className="flex flex-col h-full">
      <header className="px-6 py-4 border-b border-border">
        <Link
          to="/"
          className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Workbench
        </Link>
      </header>

      <div className="flex-1 overflow-auto px-6 py-6 space-y-6 max-w-4xl">
        {/* TL;DR Card */}
        <div className="bg-card rounded-lg border border-border p-5">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">
              TL;DR
            </h2>
            <span className={`text-2xl font-bold tabular-nums ${confidenceColor}`}>
              {(report.confidence * 100).toFixed(0)}%
            </span>
          </div>
          <p className="text-foreground text-base leading-relaxed">
            {report.summary}
          </p>
        </div>

        {/* Root Cause */}
        <div className="bg-card rounded-lg border border-border p-5">
          <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3">
            Root Cause
          </h3>
          <p className="text-foreground leading-relaxed">{report.root_cause}</p>
        </div>

        {/* Evidence */}
        {report.evidence && report.evidence.length > 0 && (
          <div className="bg-card rounded-lg border border-border p-5">
            <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3">
              Evidence
            </h3>
            <div className="space-y-3">
              {report.evidence.map((ev, idx) => (
                <div
                  key={ev.id || idx}
                  className="flex items-start gap-3 p-3 rounded-md bg-secondary/50"
                >
                  <AlertTriangle className="h-4 w-4 text-amber-400 mt-0.5 shrink-0" />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-xs font-medium text-muted-foreground uppercase">
                        {ev.type}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        score: {ev.score.toFixed(2)}
                      </span>
                    </div>
                    <p className="text-sm text-foreground">{ev.summary}</p>
                    {ev.source_url && (
                      <a
                        href={ev.source_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-xs text-ring hover:underline mt-1 inline-flex items-center gap-1"
                      >
                        View in source <ExternalLink className="h-3 w-3" />
                      </a>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Actions */}
        {report.recommendations && report.recommendations.length > 0 && (
          <div className="bg-card rounded-lg border border-border p-5">
            <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3">
              Recommended Actions
            </h3>
            <ul className="space-y-2">
              {report.recommendations.map((rec, idx) => (
                <li key={idx} className="flex items-start gap-2 text-sm text-foreground">
                  <CheckCircle2 className="h-4 w-4 text-emerald-400 mt-0.5 shrink-0" />
                  {rec}
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Feedback */}
        <FeedbackSection reportId={report.id} />
      </div>
    </div>
  )
}

function FeedbackSection({ reportId }: { reportId: number }) {
  const queryClient = useQueryClient()
  const [submitted, setSubmitted] = useState(false)

  const mutation = useMutation({
    mutationFn: (action: FeedbackRequest['action_taken']) =>
      feedback.submit(reportId, {
        rating: action === 'accepted' ? 5 : action === 'partial' ? 3 : 1,
        comment: '',
        user_id: 'web-user',
        action_taken: action,
      }),
    onSuccess: () => {
      setSubmitted(true)
      queryClient.invalidateQueries({ queryKey: ['report', reportId] })
    },
  })

  if (submitted) {
    return (
      <div className="bg-card rounded-lg border border-border p-5">
        <div className="flex items-center gap-2 text-emerald-400">
          <CheckCircle2 className="h-5 w-5" />
          <span className="text-sm font-medium">Thank you for your feedback!</span>
        </div>
      </div>
    )
  }

  return (
    <div className="bg-card rounded-lg border border-border p-5">
      <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3">
        Was this analysis helpful?
      </h3>
      <div className="flex gap-3">
        <button
          onClick={() => mutation.mutate('accepted')}
          disabled={mutation.isPending}
          className="flex items-center gap-2 px-4 py-2 rounded-md bg-emerald-500/15 text-emerald-400 border border-emerald-500/30 text-sm hover:bg-emerald-500/25 transition-colors disabled:opacity-50"
        >
          <CheckCircle2 className="h-4 w-4" />
          Accepted
        </button>
        <button
          onClick={() => mutation.mutate('partial')}
          disabled={mutation.isPending}
          className="flex items-center gap-2 px-4 py-2 rounded-md bg-amber-500/15 text-amber-400 border border-amber-500/30 text-sm hover:bg-amber-500/25 transition-colors disabled:opacity-50"
        >
          <AlertTriangle className="h-4 w-4" />
          Partial
        </button>
        <button
          onClick={() => mutation.mutate('rejected')}
          disabled={mutation.isPending}
          className="flex items-center gap-2 px-4 py-2 rounded-md bg-red-500/15 text-red-400 border border-red-500/30 text-sm hover:bg-red-500/25 transition-colors disabled:opacity-50"
        >
          <XCircle className="h-4 w-4" />
          Rejected
        </button>
      </div>
      {mutation.isError && (
        <p className="text-xs text-destructive mt-2">Failed to submit feedback: {mutation.error.message}</p>
      )}
    </div>
  )
}
