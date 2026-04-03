import { useState } from 'react'
import { Settings as SettingsIcon, ExternalLink, RefreshCw, Server, Brain, Info } from 'lucide-react'

type ConnectionStatus = 'idle' | 'testing' | 'connected' | 'error'

export function Settings() {
  const [ooStatus, setOoStatus] = useState<ConnectionStatus>('idle')
  const [ooError, setOoError] = useState<string | null>(null)

  async function testOoConnection() {
    setOoStatus('testing')
    setOoError(null)
    try {
      const res = await fetch('http://localhost:5080/health', {
        signal: AbortSignal.timeout(5000),
      })
      if (res.ok) {
        setOoStatus('connected')
      } else {
        setOoStatus('error')
        setOoError(`HTTP ${res.status}`)
      }
    } catch (err) {
      setOoStatus('error')
      setOoError(err instanceof Error ? err.message : 'Connection failed')
    }
  }

  return (
    <div className="flex flex-col h-full">
      <header className="px-6 py-4 border-b border-border">
        <h1 className="text-lg font-semibold text-foreground flex items-center gap-2">
          <SettingsIcon className="h-5 w-5 text-muted-foreground" />
          Settings
        </h1>
        <p className="text-sm text-muted-foreground mt-0.5">
          Configure observability backends and analysis preferences
        </p>
      </header>

      <div className="flex-1 overflow-auto px-6 py-6 space-y-6 max-w-3xl">
        {/* Observability Backend */}
        <section className="bg-card rounded-lg border border-border p-5">
          <div className="flex items-center gap-2 mb-4">
            <Server className="h-4 w-4 text-ring" />
            <h2 className="text-sm font-medium text-foreground">
              Observability Backend
            </h2>
          </div>

          <div className="space-y-3">
            <ConfigRow label="Provider" value="OpenObserve" />
            <ConfigRow label="Base URL" value="http://localhost:5080" />
            <ConfigRow label="Organization" value="default" />
          </div>

          <div className="mt-4 pt-4 border-t border-border flex items-center gap-4">
            <button
              onClick={testOoConnection}
              disabled={ooStatus === 'testing'}
              className="flex items-center gap-2 px-3 py-1.5 text-sm rounded-md bg-secondary text-secondary-foreground hover:bg-secondary/80 transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${ooStatus === 'testing' ? 'animate-spin' : ''}`} />
              Test Connection
            </button>

            <StatusIndicator
              status={ooStatus}
              connectedLabel="Connected"
              errorLabel={ooError ?? 'Disconnected'}
            />
          </div>
        </section>

        {/* LLM Configuration */}
        <section className="bg-card rounded-lg border border-border p-5">
          <div className="flex items-center gap-2 mb-4">
            <Brain className="h-4 w-4 text-ring" />
            <h2 className="text-sm font-medium text-foreground">
              LLM Configuration
            </h2>
          </div>

          <div className="space-y-3">
            <ConfigRow label="Provider" value="OpenAI Compatible" />
            <ConfigRow label="Base URL" value="https://api.openai.com/v1" />
            <ConfigRow label="RCA Model" value="gpt-4o (max 4096 tokens)" />
            <ConfigRow label="Summarize Model" value="gpt-4o-mini (max 1024 tokens)" />
            <ConfigRow label="Embedding Model" value="text-embedding-3-small" />
          </div>

          <p className="mt-4 pt-4 border-t border-border text-xs text-muted-foreground">
            LLM configuration is read-only. Edit <code className="text-ring">configs/local.yaml</code> and restart the server to apply changes.
          </p>
        </section>

        {/* About */}
        <section className="bg-card rounded-lg border border-border p-5">
          <div className="flex items-center gap-2 mb-4">
            <Info className="h-4 w-4 text-ring" />
            <h2 className="text-sm font-medium text-foreground">
              About
            </h2>
          </div>

          <div className="space-y-3">
            <ConfigRow label="Version" value="0.1.0-dev" />
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Description</span>
              <span className="text-sm text-foreground text-right max-w-xs">
                AI-Native Root Cause Analysis platform that compresses observability data into actionable insights
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Repository</span>
              <a
                href="https://github.com/atlanssia/aisre"
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-ring hover:underline inline-flex items-center gap-1"
              >
                github.com/atlanssia/aisre
                <ExternalLink className="h-3 w-3" />
              </a>
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}

function ConfigRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm text-foreground font-mono">{value}</span>
    </div>
  )
}

function StatusIndicator({
  status,
  connectedLabel,
  errorLabel,
}: {
  status: ConnectionStatus
  connectedLabel: string
  errorLabel: string
}) {
  if (status === 'idle' || status === 'testing') {
    return (
      <span className="text-sm text-muted-foreground">
        {status === 'testing' ? 'Testing...' : 'Not tested'}
      </span>
    )
  }

  const isConnected = status === 'connected'
  return (
    <span className="flex items-center gap-2 text-sm">
      <span
        className={`inline-block h-2 w-2 rounded-full ${
          isConnected ? 'bg-emerald-400' : 'bg-red-400'
        }`}
      />
      <span className={isConnected ? 'text-emerald-400' : 'text-red-400'}>
        {isConnected ? connectedLabel : errorLabel}
      </span>
    </span>
  )
}
