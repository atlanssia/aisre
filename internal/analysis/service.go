package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// ToolOrchestrator defines the interface for collecting evidence from observability tools.
// The consuming package (analysis) defines this interface so the concrete tool.Orchestrator
// is decoupled from the analysis engine.
type ToolOrchestrator interface {
	ExecuteAll(ctx context.Context, incident *contract.Incident) ([]contract.ToolResult, error)
}

// SimilarFinder finds similar incidents (optional dependency for Phase 2).
type SimilarFinder interface {
	FindSimilar(ctx context.Context, incidentID int64, topK int, threshold float64) ([]contract.SimilarResult, error)
}

// ChangeFinder fetches change events for correlation with incidents (Phase 2).
type ChangeFinder interface {
	GetChangesForIncident(ctx context.Context, incidentID int64) (*contract.ChangeCorrelation, error)
}

// Service defines the core analysis engine interface.
type Service interface {
	// AnalyzeIncident runs the full RCA pipeline for a given incident.
	// It fetches the incident, collects evidence via tools, and generates a report.
	AnalyzeIncident(ctx context.Context, incidentID int64) (*contract.ReportResponse, error)
}

// RCAServiceConfig holds dependencies for the RCA service.
type RCAServiceConfig struct {
	LLMClient     *LLMClient
	IncidentRepo  store.IncidentRepo
	ReportRepo    store.ReportRepo
	EvidenceRepo  store.EvidenceRepo
	Orchestrator  ToolOrchestrator // optional — if nil, AnalyzeIncident runs with no evidence
	SimilarFinder SimilarFinder   // optional — if nil, no similar incidents injected (Phase 2)
	ChangeFinder  ChangeFinder    // optional — if nil, no change correlation (Phase 2)
	Logger        *slog.Logger
}

// RCAService orchestrates the full RCA pipeline:
// build context -> rank evidence -> call LLM -> parse response -> save report.
type RCAService struct {
	llm           *LLMClient
	incidents     store.IncidentRepo
	reports       store.ReportRepo
	evidence      store.EvidenceRepo
	orchestrator  ToolOrchestrator
	similarFinder SimilarFinder
	changeFinder  ChangeFinder
	builder       *ContextBuilder
	ranker        *EvidenceRanker
	logger        *slog.Logger
}

// NewRCAService creates a new RCA service with the given configuration.
func NewRCAService(cfg RCAServiceConfig) *RCAService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &RCAService{
		llm:           cfg.LLMClient,
		incidents:     cfg.IncidentRepo,
		reports:       cfg.ReportRepo,
		evidence:      cfg.EvidenceRepo,
		orchestrator:  cfg.Orchestrator,
		similarFinder: cfg.SimilarFinder,
		changeFinder:  cfg.ChangeFinder,
		builder:       NewContextBuilder(),
		ranker:        NewEvidenceRanker(),
		logger:        cfg.Logger,
	}
}

// AnalyzeIncident runs the full RCA pipeline for an incident.
// It fetches tool results via the tool orchestrator (if configured),
// then passes them to the analysis engine.
// If the orchestrator fails or is not configured, analysis proceeds with no evidence.
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

	// Collect evidence via tool orchestrator
	var toolResults []contract.ToolResult
	if s.orchestrator != nil {
		results, err := s.orchestrator.ExecuteAll(ctx, contractInc)
		if err != nil {
			// Graceful degradation: log warning but continue with no evidence
			s.logger.Warn("tool orchestrator failed, continuing with no evidence",
				"incident_id", incidentID,
				"error", err,
			)
		} else {
			toolResults = results
		}
	}

	return s.analyze(ctx, contractInc, toolResults)
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

// analyze is the internal pipeline that orchestrates the 3-stage RCA process:
// Stage 1: Context Prompt — structure incident + environment into analysis context
// Stage 2: Evidence Prompt — rank and filter tool evidence into a prioritized chain
// Stage 3: RCA Prompt — reason about root cause, generate hypotheses and actions
func (s *RCAService) analyze(ctx context.Context, incident *contract.Incident, toolResults []contract.ToolResult) (*contract.ReportResponse, error) {
	// Rank evidence regardless of LLM stages
	rankedEvidence := s.ranker.RankAndAssign(toolResults, 10)

	// Build base context from incident + tool results
	analysisCtx := s.builder.Build(incident, toolResults)

	s.logger.Info("starting 3-stage RCA pipeline",
		"incident_id", incident.ID,
		"service", incident.ServiceName,
		"evidence_count", len(toolResults),
	)

	// Inject similar incidents (Phase 2, optional)
	var similarRCA []contract.RCAReport
	if s.similarFinder != nil {
		similarResults, err := s.similarFinder.FindSimilar(ctx, incident.ID, 3, 0.5)
		if err == nil && len(similarResults) > 0 {
			for _, r := range similarResults {
				similarRCA = append(similarRCA, contract.RCAReport{
					Summary:   r.Summary,
					RootCause: r.RootCause,
				})
			}
			s.logger.Info("injected similar incidents into RCA prompt",
				"incident_id", incident.ID,
				"similar_count", len(similarRCA),
			)
		} else if err != nil {
			s.logger.Warn("failed to find similar incidents, continuing without",
				"incident_id", incident.ID,
				"error", err,
			)
		}
	}

	// ── Stage 1: Context Prompt ──
	// Convert raw alert + environment into structured analysis context
	contextPrompt := s.buildContextPrompt(incident)
	contextMessages := []Message{
		{Role: "system", Content: "You are an observability analysis assistant. Structure the raw alert into a JSON analysis context."},
		{Role: "user", Content: contextPrompt},
	}
	contextResp, err := s.llm.Complete(ctx, contextMessages)
	if err != nil {
		return nil, fmt.Errorf("rca_service: stage 1 (context) LLM call failed: %w", err)
	}

	// ── Stage 2: Evidence Prompt ──
	// Rank and filter evidence, produce a prioritized evidence summary
	evidencePrompt := s.buildEvidencePrompt(toolResults)
	evidenceMessages := []Message{
		{Role: "system", Content: "You are an SRE evidence analyst. From the multi-source data, extract and rank the most critical evidence. Respond with a JSON array of evidence items."},
		{Role: "user", Content: evidencePrompt},
	}
	evidenceResp, err := s.llm.Complete(ctx, evidenceMessages)
	if err != nil {
		s.logger.Warn("stage 2 (evidence) LLM call failed, using ranked evidence", "error", err)
		evidenceResp = &LLMResponse{Content: "[]"} // graceful degradation
	}

	// ── Stage 3: RCA Prompt ──
	// Combine context + evidence to reason about root cause
	rcaPrompt := s.builder.BuildPrompt(analysisCtx)
	rcaPrompt = rcaPrompt + "\n\n## Structured Context (Stage 1)\n" + contextResp.Content
	rcaPrompt = rcaPrompt + "\n\n## Evidence Analysis (Stage 2)\n" + evidenceResp.Content

	// Append similar historical incidents if available
	if len(similarRCA) > 0 {
		rcaPrompt += "\n\n## Historical Similar Incidents\n"
		for i, r := range similarRCA {
			rcaPrompt += fmt.Sprintf("\n### Similar Incident %d\n- **Summary:** %s\n- **Root Cause:** %s\n",
				i+1, r.Summary, r.RootCause)
		}
	}

	// Append change correlation if available (Phase 2)
	if s.changeFinder != nil {
		changeCorr, err := s.changeFinder.GetChangesForIncident(ctx, incident.ID)
		if err == nil && changeCorr != nil && len(changeCorr.Changes) > 0 {
			rcaPrompt += "\n\n## Recent Changes (Correlated)\n"
			rcaPrompt += "The following change events occurred around the incident time window:\n\n"
			for _, ch := range changeCorr.Changes {
				rcaPrompt += fmt.Sprintf("- [%s] %s: %s (by %s) at %s\n",
					ch.ChangeType, ch.Service, ch.Summary, ch.Author, ch.Timestamp)
			}
			s.logger.Info("injected change correlation into RCA prompt",
				"incident_id", incident.ID,
				"change_count", len(changeCorr.Changes),
			)
		} else if err != nil {
			s.logger.Warn("failed to fetch change correlation, continuing without",
				"incident_id", incident.ID,
				"error", err,
			)
		}
	}

	rcaMessages := []Message{
		{Role: "system", Content: "You are a senior SRE Root Cause Analyst. You must follow evidence and avoid unsupported assumptions. Respond with valid JSON only.\n\nYour response MUST include:\n- \"actions\": {\"immediate\": [...], \"fix\": [...], \"prevention\": [...]}\n- \"timeline\": an array of events reconstructing the fault chronology.\nEach timeline entry: {\"time\": \"ISO8601\", \"type\": \"symptom|error|deploy|alert|action\", \"service\": \"...\", \"description\": \"...\", \"severity\": \"info|warning|error|critical\"}"},
		{Role: "user", Content: rcaPrompt},
	}

	llmResp, err := s.llm.Complete(ctx, rcaMessages)
	if err != nil {
		return nil, fmt.Errorf("rca_service: stage 3 (RCA) LLM call failed: %w", err)
	}

	// Parse final RCA output
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
	recommendations = append(recommendations, rcaOutput.Actions.Fix...)
	recommendations = append(recommendations, rcaOutput.Actions.Prevention...)

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

	// Build timeline from LLM output
	timeline := make([]contract.TimelineEvent, len(rcaOutput.Timeline))
	for i, te := range rcaOutput.Timeline {
		timeline[i] = contract.TimelineEvent{
			Time:        te.Time,
			Type:        te.Type,
			Service:     te.Service,
			Description: te.Description,
			Severity:    te.Severity,
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
		Timeline:        timeline,
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

	// Parse recommendations and timeline from stored JSON
	var recommendations []string
	var timeline []contract.TimelineEvent
	if report.ReportJSON != "" {
		var stored struct {
			Actions struct {
				Immediate  []string `json:"immediate"`
				Fix        []string `json:"fix"`
				Prevention []string `json:"prevention"`
			} `json:"actions"`
			Timeline []contract.TimelineEvent `json:"timeline"`
		}
		if json.Unmarshal([]byte(report.ReportJSON), &stored) == nil {
			recommendations = append(recommendations, stored.Actions.Immediate...)
			recommendations = append(recommendations, stored.Actions.Fix...)
			recommendations = append(recommendations, stored.Actions.Prevention...)
			timeline = stored.Timeline
		}
	}
	if recommendations == nil {
		recommendations = []string{}
	}
	if timeline == nil {
		timeline = []contract.TimelineEvent{}
	}

	return &contract.ReportResponse{
		ID:              report.ID,
		IncidentID:      report.IncidentID,
		Summary:         report.Summary,
		RootCause:       report.RootCause,
		Confidence:      report.Confidence,
		Evidence:        evidence,
		Recommendations: recommendations,
		Timeline:        timeline,
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

// buildContextPrompt creates the Stage 1 prompt that structures raw alert data.
func (s *RCAService) buildContextPrompt(incident *contract.Incident) string {
	return fmt.Sprintf(`Analyze this alert and produce a structured JSON context:

Service: %s
Severity: %s
Source: %s
TraceID: %s
Time: %s

Output JSON:
{
  "service": "...",
  "anomaly_type": "...",
  "time_window": {"start": "...", "end": "..."},
  "affected_services": [],
  "context_signals": []
}`,
		incident.ServiceName, incident.Severity, incident.Source,
		incident.TraceID, incident.CreatedAt,
	)
}

// buildEvidencePrompt creates the Stage 2 prompt that ranks evidence.
func (s *RCAService) buildEvidencePrompt(toolResults []contract.ToolResult) string {
	if len(toolResults) == 0 {
		return "No tool evidence was collected. Return an empty JSON array: []"
	}

	var b strings.Builder
	b.WriteString("From the following multi-source observability data, extract and rank the most critical evidence.\n\n")

	for i, tr := range toolResults {
		payloadJSON, _ := json.Marshal(tr.Payload)
		fmt.Fprintf(&b, "### Evidence %d: %s (score: %.2f)\n%s\nDetails: %s\n\n",
			i+1, tr.Name, tr.Score, tr.Summary, string(payloadJSON))
	}

	b.WriteString("For each evidence item, output JSON: {\"evidence_id\": \"ev_NNN\", \"type\": \"...\", \"score\": 0.0-1.0, \"summary\": \"...\", \"why_important\": \"...\"}\n")
	b.WriteString("Sort by score descending. Output a JSON array.")
	return b.String()
}
