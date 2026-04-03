package similar

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/atlanssia/aisre/internal/analysis"
	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// Service provides business logic for computing incident embeddings and
// finding similar incidents via cosine similarity search.
type Service struct {
	embClient *analysis.EmbeddingClient
	embRepo   store.EmbeddingRepo
	incRepo   store.IncidentRepo
	rptRepo   store.ReportRepo
}

// NewService creates a new SimilarService with the given dependencies.
func NewService(
	embClient *analysis.EmbeddingClient,
	embRepo store.EmbeddingRepo,
	incRepo store.IncidentRepo,
	rptRepo store.ReportRepo,
) *Service {
	return &Service{
		embClient: embClient,
		embRepo:   embRepo,
		incRepo:   incRepo,
		rptRepo:   rptRepo,
	}
}

// ComputeEmbedding generates an embedding vector for the given incident by
// concatenating the incident service name with the report summary and root
// cause, then stores the result via the embedding repository.
func (s *Service) ComputeEmbedding(ctx context.Context, incidentID int64) error {
	inc, err := s.incRepo.GetByID(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("similar: get incident: %w", err)
	}

	reports, err := s.rptRepo.List(ctx, store.ReportFilter{Limit: 1000})
	if err != nil {
		return fmt.Errorf("similar: list reports: %w", err)
	}

	var report *store.Report
	for i := range reports {
		if reports[i].IncidentID == incidentID {
			report = &reports[i]
			break
		}
	}
	if report == nil {
		return fmt.Errorf("similar: no report found for incident %d", incidentID)
	}

	text := inc.ServiceName + " " + report.Summary + " " + report.RootCause
	slog.Info("computing embedding", "incident_id", incidentID, "text_length", len(text))

	vectors, err := s.embClient.Embed(ctx, []string{text})
	if err != nil {
		return fmt.Errorf("similar: embed: %w", err)
	}
	if len(vectors) == 0 {
		return fmt.Errorf("similar: embed returned no vectors")
	}

	encoded := EncodeVector(vectors[0])
	if err := s.embRepo.Create(ctx, incidentID, inc.ServiceName, encoded, s.embClient.Model()); err != nil {
		return fmt.Errorf("similar: store embedding: %w", err)
	}

	slog.Info("embedding stored", "incident_id", incidentID, "model", s.embClient.Model())
	return nil
}

// FindSimilar searches for incidents similar to the given one. It loads the
// query embedding, retrieves candidate embeddings from the same service,
// computes cosine similarity, enriches results with incident/report metadata,
// and returns the top-K results above the given threshold.
func (s *Service) FindSimilar(ctx context.Context, incidentID int64, topK int, threshold float64) ([]contract.SimilarResult, error) {
	queryEmb, err := s.embRepo.GetByIncidentID(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("similar: get query embedding: %w", err)
	}

	queryVec := DecodeVector(queryEmb.Embedding)

	candidates, err := s.embRepo.ListByService(ctx, queryEmb.Service)
	if err != nil {
		return nil, fmt.Errorf("similar: list candidates: %w", err)
	}

	type scored struct {
		incidentID int64
		similarity float64
		service    string
	}

	var scoredList []scored
	for _, cand := range candidates {
		if cand.IncidentID == incidentID {
			continue // skip self
		}
		candVec := DecodeVector(cand.Embedding)
		sim := CosineSimilarity(queryVec, candVec)
		if sim < threshold {
			continue
		}
		scoredList = append(scoredList, scored{
			incidentID: cand.IncidentID,
			similarity: sim,
			service:    cand.Service,
		})
	}

	sort.Slice(scoredList, func(i, j int) bool {
		return scoredList[i].similarity > scoredList[j].similarity
	})

	if topK > 0 && len(scoredList) > topK {
		scoredList = scoredList[:topK]
	}

	// Enrich results with incident severity and report summary/root_cause.
	results := make([]contract.SimilarResult, 0, len(scoredList))
	for _, sc := range scoredList {
		inc, err := s.incRepo.GetByID(ctx, sc.incidentID)
		if err != nil {
			slog.Warn("skip candidate: incident not found", "incident_id", sc.incidentID, "error", err)
			continue
		}

		reports, err := s.rptRepo.List(ctx, store.ReportFilter{Limit: 1000})
		if err != nil {
			slog.Warn("skip candidate: list reports failed", "error", err)
			continue
		}

		var summary, rootCause string
		for i := range reports {
			if reports[i].IncidentID == sc.incidentID {
				summary = reports[i].Summary
				rootCause = reports[i].RootCause
				break
			}
		}

		results = append(results, contract.SimilarResult{
			IncidentID: sc.incidentID,
			Similarity: sc.similarity,
			Summary:    summary,
			RootCause:  rootCause,
			Service:    sc.service,
			Severity:   inc.Severity,
		})
	}

	return results, nil
}
