import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { reports } from '@/api/client'
import { Search, FileText, ExternalLink } from 'lucide-react'
import { Link } from 'react-router-dom'
import type { RCAReport } from '@/types'

export function History() {
  const [query, setQuery] = useState('')
  const [searchTerm, setSearchTerm] = useState('')

  const { data, isLoading, error } = useQuery({
    queryKey: ['reports', 'search', searchTerm],
    queryFn: () => reports.search({ q: searchTerm }),
    enabled: !!searchTerm,
  })

  const items: RCAReport[] = data ?? []

  function handleSearch(e: React.FormEvent) {
    e.preventDefault()
    setSearchTerm(query)
  }

  return (
    <div className="flex flex-col h-full">
      <header className="px-6 py-4 border-b border-border">
        <h1 className="text-lg font-semibold text-foreground">History</h1>
        <p className="text-sm text-muted-foreground mt-0.5">
          Search and review past RCA reports
        </p>
      </header>

      {/* Search Bar */}
      <div className="px-6 py-3 border-b border-border">
        <form onSubmit={handleSearch} className="flex gap-2">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search reports by keyword..."
              className="w-full pl-10 pr-4 py-2 rounded-md border border-border bg-background text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
          <button
            type="submit"
            className="px-4 py-2 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
          >
            Search
          </button>
        </form>
      </div>

      {/* Results */}
      <div className="flex-1 overflow-auto px-6 py-3">
        {!searchTerm ? (
          <div className="flex flex-col items-center justify-center h-64 text-muted-foreground">
            <Search className="h-8 w-8 mb-2 opacity-50" />
            <p>Enter a search term to find RCA reports</p>
          </div>
        ) : isLoading ? (
          <div className="flex items-center justify-center h-64 text-muted-foreground">
            Searching...
          </div>
        ) : error ? (
          <div className="flex items-center justify-center h-64 text-destructive">
            Search failed: {error.message}
          </div>
        ) : items.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64 text-muted-foreground">
            <FileText className="h-8 w-8 mb-2 opacity-50" />
            <p>No reports found matching &quot;{searchTerm}&quot;</p>
          </div>
        ) : (
          <div className="space-y-2">
            {items.map((report) => (
              <Link
                key={report.id}
                to={`/reports/${report.id}`}
                className="flex items-center gap-4 px-4 py-3 rounded-lg hover:bg-secondary/50 transition-colors group"
              >
                <FileText className="h-4 w-4 text-muted-foreground shrink-0" />
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-foreground font-medium truncate">
                    {report.summary || `Report #${report.id}`}
                  </p>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    Incident #{report.incident_id} &middot; Confidence: {(report.confidence * 100).toFixed(0)}%
                  </p>
                </div>
                <ExternalLink className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
