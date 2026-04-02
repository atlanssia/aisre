package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// Service defines the core analysis engine interface.
type Service interface {
	// AnalyzeIncident runs the full RCA pipeline for a given incident.
	// It fetches the incident, collects evidence via tools, and generates a report.
	AnalyzeIncident(ctx context.Context, incidentID int64) (*contract.ReportResponse, error)
}

// RCAServiceConfig holds dependencies for the RCA service.
type RCAServiceConfig struct {
	LLMClient    *LLMClient
	IncidentRepo store.IncidentRepo
	ReportRepo   store.ReportRepo
	EvidenceRepo store.EvidenceRepo
	Logger       *slog.Logger
}

// RCAService orchestrates the full RCA pipeline:
// build context -> rank evidence -> call LLM -> parse response -> save report.
type RCAService struct {
	llm       *LLMClient
	incidents store.IncidentRepo
	reports   store.ReportRepo
	evidence  store.EvidenceRepo
	builder   *ContextBuilder
	ranker    *EvidenceRanker
	logger    *slog.Logger
}

// NewRCAService creates a new RCA service with the given configuration.
func NewRCAService(cfg RCAServiceConfig) *RCAService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &RCAService{
		llm:       cfg.LLMClient,
		incidents: cfg.IncidentRepo,
		reports:   cfg.ReportRepo,
		evidence:  cfg.EvidenceRepo,
		builder:   NewContextBuilder(),
		ranker:    NewEvidenceRanker(),
		logger:    cfg.Logger,
	}
}

// AnalyzeIncident runs the full RCA pipeline for an incident.
// Note: This method fetches tool results via the tool orchestrator.
// For testing and direct use, use AnalyzeIncidentWithEvidence.
func (s *RCAService) AnalyzeIncident(ctx context.Context, incidentID int64) (*contract.ReportResponse, error) {
	// Verify incident exists
	inc, err := s.incidents.GetByID(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("rca_service: get incident: %w", err)
	}

	// Convert store incident to contract incident
	contractInc := &contract.Incident{
		ID:          inc.ID,
		Source:      inc.Source,
		ServiceName: inc.ServiceName,
		Severity:    inc.Severity,
		Status:      inc.Status,
		TraceID:     inc.TraceID,
		CreatedAt:   inc.CreatedAt,
	}

	// Build context with no tool results (tools not integrated yet)
	return s.analyze(ctx, contractInc, nil)
}

// AnalyzeIncidentWithEvidence runs RCA with provided tool results.
// This is the primary method for external callers who have already collected evidence.
func (s *RCAService) AnalyzeIncidentWithEvidence(ctx context.Context, incidentID int64, toolResults []contract.ToolResult) (*contract.ReportResponse, error) {
	inc, err := s.incidents.GetByID(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("rca_service: get incident: %w", err)
	}

	contractInc := &contract.Incident{
		ID:          inc.ID,
		Source:      inc.Source,
		ServiceName: inc.ServiceName,
		Severity:    inc.Severity,
		Status:      inc.Status,
		TraceID:     inc.TraceID,
		CreatedAt:   inc.CreatedAt,
	}

	return s.analyze(ctx, contractInc, toolResults)
}

// analyze is the internal pipeline that orchestrates the RCA process.
func (s *RCAService) analyze(ctx context.Context, incident *contract.Incident, toolResults []contract.ToolResult) (*contract.ReportResponse, error) {
	// Step 1: Rank evidence
	rankedEvidence := s.ranker.RankAndAssign(toolResults, 10)

	// Step 2: Build context for LLM
	analysisCtx := s.builder.Build(incident, toolResults)
	prompt := s.builder.BuildPrompt(analysisCtx)

	// Step 3: Call LLM
	s.logger.Info("calling LLM for RCA analysis",
		"incident_id", incident.ID,
		"service", incident.ServiceName,
		"evidence_count", len(toolResults),
	)

	messages := []Message{
		{Role: "system", Content: "You are an expert Site Reliability Engineer. Analyze the incident and respond with valid JSON only."},
		{Role: "user", Content: prompt},
	}

	llmResp, err := s.llm.Complete(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("rca_service: LLM call failed: %w", err)
	}

	// Step 4: Parse LLM response
	rcaOutput, err := s.llm.ParseRCAOutput(llmResp.Content)
	if err != nil {
		return nil, fmt.Errorf("rca_service: parse RCA output: %w", err)
	}

	// Step 5: Save report
	rcaJSON, _ := json.Marshal(rcaOutput)
	report := &store.Report{
		IncidentID: incident.ID,
		Summary:    rcaOutput.Summary,
		RootCause:  rcaOutput.RootCause,
		Confidence: rcaOutput.Confidence,
		ReportJSON: string(rcaJSON),
	}

	reportID, err := s.reports.Create(ctx, report)
	if err != nil {
		return nil, fmt.Errorf("rca_service: save report: %w", err)
	}

	// Step 6: Save evidence items
	for _, ev := range rankedEvidence {
		payloadJSON, _ := json.Marshal(ev.Payload)
		evidenceItem := &store.Evidence{
			ReportID:     reportID,
			EvidenceType: ev.Type,
			Score:        ev.Score,
			Payload:      string(payloadJSON),
			SourceURL:    ev.SourceURL,
		}
		if _, err := s.evidence.Create(ctx, evidenceItem); err != nil {
			s.logger.Warn("failed to save evidence item", "error", err, "type", ev.Type)
		}
	}

	// Step 7: Build response
	recommendations := rcaOutput.Actions.Immediate
	recommendations = append(recommendations, rcaOutput.Actions.ShortTerm...)
	recommendations = append(recommendations, rcaOutput.Actions.LongTerm...)

	s.logger.Info("RCA analysis complete",
		"report_id", reportID,
		"incident_id", incident.ID,
		"confidence", rcaOutput.Confidence,
	)

	// Build evidence items for response
	contractEvidence := make([]contract.EvidenceItem, len(rankedEvidence))
	for i, ev := range rankedEvidence {
		contractEvidence[i] = contract.EvidenceItem{
			ID:      ev.ID,
			Type:    ev.Type,
			Score:   ev.Score,
			Summary: ev.Summary,
			Payload: ev.Payload,
		}
	}

	return &contract.ReportResponse{
		ID:              reportID,
		IncidentID:      incident.ID,
		Summary:         rcaOutput.Summary,
		RootCause:       rcaOutput.RootCause,
		Confidence:      rcaOutput.Confidence,
		Evidence:        contractEvidence,
		Recommendations: recommendations,
	}, nil
}

// GetReport retrieves a report by ID with its evidence items.
func (s *RCAService) GetReport(ctx context.Context, reportID int64) (*contract.ReportResponse, error) {
	report, err := s.reports.GetByID(ctx, reportID)
	if err != nil {
		return nil, fmt.Errorf("rca_service: get report: %w", err)
	}

	evidenceItems, err := s.evidence.ListByReport(ctx, reportID)
	if err != nil {
		return nil, fmt.Errorf("rca_service: get evidence: %w", err)
	}

	evidence := make([]contract.EvidenceItem, len(evidenceItems))
	for i, ev := range evidenceItems {
		var payload map[string]any
		if ev.Payload != "" {
			json.Unmarshal([]byte(ev.Payload), &payload)
		}
		evidence[i] = contract.EvidenceItem{
			Type:      ev.EvidenceType,
			Score:     ev.Score,
			SourceURL: ev.SourceURL,
			Payload:   payload,
		}
	}

	return &contract.ReportResponse{
		ID:              report.ID,
		IncidentID:      report.IncidentID,
		Summary:         report.Summary,
		RootCause:       report.RootCause,
		Confidence:      report.Confidence,
		Evidence:        evidence,
		Recommendations: []string{},
		CreatedAt:       report.CreatedAt,
	}, nil
}

// GetEvidence retrieves evidence items for a report.
func (s *RCAService) GetEvidence(ctx context.Context, reportID int64) ([]contract.EvidenceItem, error) {
	items, err := s.evidence.ListByReport(ctx, reportID)
	if err != nil {
		return nil, fmt.Errorf("rca_service: get evidence: %w", err)
	}

	result := make([]contract.EvidenceItem, len(items))
	for i, ev := range items {
		var payload map[string]any
		if ev.Payload != "" {
			json.Unmarshal([]byte(ev.Payload), &payload)
		}
		result[i] = contract.EvidenceItem{
			Type:      ev.EvidenceType,
			Score:     ev.Score,
			SourceURL: ev.SourceURL,
			Payload:   payload,
		}
	}
	return result, nil
}
