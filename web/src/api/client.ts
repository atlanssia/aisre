import type {
  Incident,
  CreateIncidentRequest,
  CreateIncidentResponse,
  WebhookPayload,
  RCAReport,
  FeedbackRequest,
  FeedbackResponse,
} from '@/types'

const API_BASE = '/api/v1'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Unknown error', code: 'UNKNOWN' }))
    throw new ApiError(err.error, err.code, res.status)
  }

  return res.json()
}

export class ApiError extends Error {
  constructor(
    message: string,
    public code: string,
    public status: number,
  ) {
    super(message)
  }
}

// Incidents
export const incidents = {
  list: (params?: { service?: string; severity?: string; status?: string; limit?: number }) => {
    const qs = new URLSearchParams()
    if (params?.service) qs.set('service', params.service)
    if (params?.severity) qs.set('severity', params.severity)
    if (params?.status) qs.set('status', params.status)
    if (params?.limit) qs.set('limit', String(params.limit))
    const query = qs.toString()
    return request<Incident[]>(`/incidents${query ? `?${query}` : ''}`)
  },

  get: (id: number) => request<Incident>(`/incidents/${id}`),

  create: (data: CreateIncidentRequest) =>
    request<CreateIncidentResponse>('/incidents', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
}

// Alerts Webhook
export const alerts = {
  webhook: (data: WebhookPayload) =>
    request<CreateIncidentResponse>('/alerts/webhook', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
}

// Reports
export const reports = {
  get: (id: number) => request<RCAReport>(`/reports/${id}`),

  analyze: (incidentId: number) =>
    request<RCAReport>(`/incidents/${incidentId}/analyze`, {
      method: 'POST',
    }),

  search: (params: { q?: string; service?: string; severity?: string; date_range?: string }) => {
    const qs = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => { if (v) qs.set(k, v) })
    return request<RCAReport[]>(`/reports/search?${qs}`)
  },
}

// Feedback
export const feedback = {
  submit: (reportId: number, data: FeedbackRequest) =>
    request<FeedbackResponse>(`/reports/${reportId}/feedback`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  list: (reportId: number) =>
    request<FeedbackResponse[]>(`/reports/${reportId}/feedback`),
}
