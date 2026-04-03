export interface Incident {
  id: number
  source: string
  service_name: string
  severity: Severity
  status: IncidentStatus
  trace_id?: string
  created_at: string
}

export type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info'
export type IncidentStatus = 'open' | 'analyzing' | 'resolved' | 'closed'

export interface CreateIncidentRequest {
  source: string
  service: string
  severity: Severity
  time_range?: string
  trace_id?: string
}

export interface CreateIncidentResponse {
  incident_id: number
  report_id?: number
  status: string
}

export interface WebhookPayload {
  source: string
  alert_name: string
  service: string
  severity: Severity
  trace_id?: string
  payload?: Record<string, unknown>
}

export interface RCAReport {
  id: number
  incident_id: number
  summary: string
  root_cause: string
  confidence: number
  evidence: EvidenceItem[]
  recommendations: string[]
  created_at: string
}

export interface EvidenceItem {
  id: string
  type: 'trace' | 'log' | 'metric' | 'change'
  score: number
  summary: string
  source_url?: string
  payload: Record<string, unknown>
}

export interface FeedbackRequest {
  rating: number
  comment: string
  user_id: string
  action_taken: 'accepted' | 'partial' | 'rejected'
}

export interface FeedbackResponse {
  id: number
  report_id: number
  rating: number
  comment: string
  user_id: string
  action_taken: string
  created_at: string
}

export interface ApiError {
  error: string
  code: string
}

export interface SSEEvent {
  event: string
  data: unknown
}

export interface AnalysisProgressEvent {
  percent: number
  phase: string
}

export interface AnalysisStatusEvent {
  message: string
  phase: string
}
