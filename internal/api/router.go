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

// SimilarService defines the interface for computing embeddings and finding similar incidents.
type SimilarService interface {
	ComputeEmbedding(ctx context.Context, incidentID int64) error
	FindSimilar(ctx context.Context, incidentID int64, topK int, threshold float64) ([]contract.SimilarResult, error)
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
	return NewRouterFullWithSimilar(svc, analysisSvc, feedbackRepo, reportRepo, nil, staticFS)
}

// NewRouterFullWithSimilar creates a router with all endpoints including similar incident search.
func NewRouterFullWithSimilar(svc IncidentService, analysisSvc AnalysisService, feedbackRepo store.FeedbackRepo, reportRepo store.ReportRepo, similarSvc SimilarService, staticFS fs.FS) http.Handler {
	return NewRouterFullWithChanges(svc, analysisSvc, feedbackRepo, reportRepo, similarSvc, nil, staticFS)
}

// NewRouterFullWithChanges creates a router with all endpoints including change correlation.
func NewRouterFullWithChanges(svc IncidentService, analysisSvc AnalysisService, feedbackRepo store.FeedbackRepo, reportRepo store.ReportRepo, similarSvc SimilarService, changeSvc ChangeService, staticFS fs.FS) http.Handler {
	return NewRouterFullWithTopology(svc, analysisSvc, feedbackRepo, reportRepo, similarSvc, changeSvc, nil, staticFS)
}

// NewRouterFullWithTopology creates a router with all endpoints including topology/blast radius.
func NewRouterFullWithTopology(svc IncidentService, analysisSvc AnalysisService, feedbackRepo store.FeedbackRepo, reportRepo store.ReportRepo, similarSvc SimilarService, changeSvc ChangeService, topoSvc TopologyService, staticFS fs.FS) http.Handler {
	return NewRouterFullWithPromptStudio(svc, analysisSvc, feedbackRepo, reportRepo, similarSvc, changeSvc, topoSvc, nil, staticFS)
}

// NewRouterFullWithPromptStudio creates a router with all endpoints including prompt studio.
func NewRouterFullWithPromptStudio(svc IncidentService, analysisSvc AnalysisService, feedbackRepo store.FeedbackRepo, reportRepo store.ReportRepo, similarSvc SimilarService, changeSvc ChangeService, topoSvc TopologyService, promptStudioSvc PromptStudioService, staticFS fs.FS) http.Handler {
	return NewRouterFullWithAlertGroup(svc, analysisSvc, feedbackRepo, reportRepo, similarSvc, changeSvc, topoSvc, promptStudioSvc, nil, staticFS)
}

// NewRouterFullWithAlertGroup creates a router with all endpoints including alert aggregation.
func NewRouterFullWithAlertGroup(svc IncidentService, analysisSvc AnalysisService, feedbackRepo store.FeedbackRepo, reportRepo store.ReportRepo, similarSvc SimilarService, changeSvc ChangeService, topoSvc TopologyService, promptStudioSvc PromptStudioService, alertGroupSvc AlertGroupService, staticFS fs.FS) http.Handler {
	return NewRouterFullWithPostmortem(svc, analysisSvc, feedbackRepo, reportRepo, similarSvc, changeSvc, topoSvc, promptStudioSvc, alertGroupSvc, nil, staticFS)
}

// NewRouterFullWithPostmortem creates a router with all endpoints including postmortem.
func NewRouterFullWithPostmortem(svc IncidentService, analysisSvc AnalysisService, feedbackRepo store.FeedbackRepo, reportRepo store.ReportRepo, similarSvc SimilarService, changeSvc ChangeService, topoSvc TopologyService, promptStudioSvc PromptStudioService, alertGroupSvc AlertGroupService, postmortemSvc PostmortemService, staticFS fs.FS) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)

	h := &handler{svc: svc, analysisSvc: analysisSvc, feedbackRepo: feedbackRepo, reportRepo: reportRepo, similarSvc: similarSvc, changeSvc: changeSvc, topoSvc: topoSvc, promptStudioSvc: promptStudioSvc, alertGroupSvc: alertGroupSvc, postmortemSvc: postmortemSvc}

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
			r.Get("/reports/{id}/feedback", h.listFeedback)
			r.Get("/reports/search", h.searchReports)

			// Similar incident routes (feature-flagged)
			if h.similarSvc != nil {
				r.Get("/incidents/{id}/similar", h.getSimilar)
				r.Post("/incidents/{id}/embed", h.computeEmbedding)
			}

			// Change correlation routes (feature-flagged)
			if h.changeSvc != nil {
				r.Get("/changes", h.listChanges)
				r.Get("/incidents/{id}/changes", h.getChangesForIncident)
				r.Post("/changes", h.ingestChange)
			}

			// Topology / Blast Radius routes (feature-flagged)
			if h.topoSvc != nil {
				r.Get("/topology", h.getTopology)
				r.Get("/incidents/{id}/blast-radius", h.getBlastRadius)
				r.Post("/topology/edges", h.addTopologyEdge)
			}

			// Prompt Studio routes (feature-flagged)
			if h.promptStudioSvc != nil {
				r.Get("/prompts", h.listPromptTemplates)
				r.Get("/prompts/{id}", h.getPromptTemplate)
				r.Post("/prompts", h.createPromptTemplate)
				r.Put("/prompts/{id}", h.updatePromptTemplate)
				r.Post("/prompts/{id}/test", h.dryRunPromptTemplate)
			}

			// Alert Group routes (feature-flagged)
			if h.alertGroupSvc != nil {
				r.Post("/alerts", h.ingestAlert)
				r.Get("/alert-groups", h.listAlertGroups)
				r.Get("/alert-groups/{id}", h.getAlertGroup)
				r.Post("/alert-groups/{id}/escalate", h.escalateAlertGroup)
			}

			// Postmortem routes (feature-flagged)
			if h.postmortemSvc != nil {
				r.Post("/incidents/{id}/postmortem", h.generatePostmortem)
				r.Get("/postmortems", h.listPostmortems)
				r.Get("/postmortems/{id}", h.getPostmortem)
				r.Put("/postmortems/{id}", h.updatePostmortem)
			}
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
	svc             IncidentService
	analysisSvc     AnalysisService
	feedbackRepo    store.FeedbackRepo
	reportRepo      store.ReportRepo
	similarSvc      SimilarService
	changeSvc       ChangeService
	topoSvc         TopologyService
	promptStudioSvc PromptStudioService
	alertGroupSvc   AlertGroupService
	postmortemSvc   PostmortemService
}

func contentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
