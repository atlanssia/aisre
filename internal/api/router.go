package api

import (
	"context"
	"io/fs"
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
	return NewRouterWithFeedback(svc, analysisSvc, nil)
}

// NewRouterWithFeedback creates a router with incident, analysis, and feedback endpoints.
func NewRouterWithFeedback(svc IncidentService, analysisSvc AnalysisService, feedbackRepo store.FeedbackRepo) http.Handler {
	return NewRouterFull(svc, analysisSvc, feedbackRepo, nil, nil)
}

// NewRouterFull creates a router with all endpoints including report search.
// staticFS is optional; if non-nil, SPA static files will be served for non-API routes.
func NewRouterFull(svc IncidentService, analysisSvc AnalysisService, feedbackRepo store.FeedbackRepo, reportRepo store.ReportRepo, staticFS fs.FS) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)

	h := &handler{svc: svc, analysisSvc: analysisSvc, feedbackRepo: feedbackRepo, reportRepo: reportRepo}

	r.Route("/api/v1", func(r chi.Router) {
		// SSE route — no contentTypeJSON middleware
		r.Get("/incidents/{id}/analyze/stream", h.streamAnalysis)

		// JSON API routes
		r.Group(func(r chi.Router) {
			r.Use(contentTypeJSON)
			r.Post("/incidents", h.createIncident)
			r.Get("/incidents", h.listIncidents)
			r.Get("/incidents/{id}", h.getIncident)
			r.Post("/incidents/{id}/analyze", h.analyzeIncident)
			r.Post("/alerts/webhook", h.handleWebhook)
			r.Get("/reports/{id}", h.getReport)
			r.Get("/reports/{id}/evidence", h.getEvidence)
			r.Post("/reports/{id}/feedback", h.submitFeedback)
			r.Get("/reports/search", h.searchReports)
		})
	})

	r.Get("/health", h.health)

	// Serve SPA static files if a filesystem is provided
	if staticFS != nil {
		spa, err := NewSPAHandler(staticFS)
		if err == nil {
			// Serve static assets directly
			r.Handle("/assets/*", http.FileServerFS(staticFS))
			// SPA fallback for all other non-API routes
			r.NotFound(spa.ServeHTTP)
		}
	}

	return r
}

type handler struct {
	svc         IncidentService
	analysisSvc AnalysisService
	feedbackRepo store.FeedbackRepo
	reportRepo   store.ReportRepo
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
