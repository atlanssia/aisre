package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// IncidentService defines the interface the API layer depends on.
type IncidentService interface {
	CreateIncident(ctx context.Context, req contract.CreateIncidentRequest) (*contract.CreateIncidentResponse, error)
	GetIncident(ctx context.Context, id int64) (*contract.Incident, error)
	ListIncidents(ctx context.Context, filter store.IncidentFilter) ([]contract.Incident, error)
	ProcessWebhook(ctx context.Context, payload contract.WebhookPayload) (*contract.CreateIncidentResponse, error)
}

// AnalysisService defines the interface for the RCA analysis API layer.
type AnalysisService interface {
	AnalyzeIncident(ctx context.Context, incidentID int64) (*contract.ReportResponse, error)
	GetReport(ctx context.Context, reportID int64) (*contract.ReportResponse, error)
	GetEvidence(ctx context.Context, reportID int64) ([]contract.EvidenceItem, error)
}

func NewRouter(svc IncidentService) http.Handler {
	return NewRouterWithAnalysis(svc, nil)
}

// NewRouterWithAnalysis creates a router with both incident and analysis endpoints.
func NewRouterWithAnalysis(svc IncidentService, analysisSvc AnalysisService) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(contentTypeJSON)

	h := &handler{svc: svc, analysisSvc: analysisSvc}

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/incidents", h.createIncident)
		r.Get("/incidents", h.listIncidents)
		r.Get("/incidents/{id}", h.getIncident)
		r.Post("/incidents/{id}/analyze", h.analyzeIncident)
		r.Post("/alerts/webhook", h.handleWebhook)
		r.Get("/reports/{id}", h.getReport)
		r.Get("/reports/{id}/evidence", h.getEvidence)
	})

	r.Get("/health", h.health)

	return r
}

type handler struct {
	svc         IncidentService
	analysisSvc AnalysisService
}

func contentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte(`{"status":"ok"}`))
}
