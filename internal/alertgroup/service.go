package alertgroup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// IncidentCreator defines the interface needed from incident service for escalation.
type IncidentCreator interface {
	CreateIncident(ctx context.Context, req contract.CreateIncidentRequest) (*contract.CreateIncidentResponse, error)
}

// Service aggregates alerts into deduplicated alert groups.
type Service struct {
	alertRepo   store.AlertGroupRepo
	incidentSvc IncidentCreator
	logger      *slog.Logger
}

// NewService creates a new alert aggregation service.
func NewService(alertRepo store.AlertGroupRepo, incidentSvc IncidentCreator) *Service {
	return &Service{
		alertRepo:   alertRepo,
		incidentSvc: incidentSvc,
		logger:      slog.Default(),
	}
}

// Ingest receives an alert, deduplicates it using fingerprint (SHA256 of sorted labels).
func (s *Service) Ingest(ctx context.Context, alert contract.IncomingAlert) (*contract.AlertGroup, error) {
	if alert.Title == "" {
		return nil, fmt.Errorf("alertgroup: ingest: title is required")
	}
	if alert.Severity == "" {
		alert.Severity = "warning"
	}
	validSeverities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true, "info": true, "warning": true}
	if !validSeverities[alert.Severity] {
		return nil, fmt.Errorf("alertgroup: ingest: invalid severity %q", alert.Severity)
	}
	fp := computeFingerprint(alert.Labels)

	existing, err := s.alertRepo.GetByFingerprint(ctx, fp)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("alertgroup: ingest: lookup: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		// Not found — create new group
		now := time.Now().UTC().Format(time.RFC3339)
		labelsJSON, _ := json.Marshal(alert.Labels)
		group := &store.AlertGroup{
			Fingerprint: fp,
			Title:       alert.Title,
			Severity:    alert.Severity,
			Labels:      string(labelsJSON),
			Count:       1,
			FirstSeen:   now,
			LastSeen:    now,
		}
		id, createErr := s.alertRepo.Create(ctx, group)
		if createErr != nil {
			return nil, fmt.Errorf("alertgroup: create: %w", createErr)
		}
		group.ID = id
		out := storeToContract(group)
		return &out, nil
	}

	// Found — update existing group
	now := time.Now().UTC().Format(time.RFC3339)
	existing.Count++
	existing.LastSeen = now
	existing.Title = alert.Title
	existing.Severity = alert.Severity
	if err := s.alertRepo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("alertgroup: update: %w", err)
	}
	out := storeToContract(existing)
	return &out, nil
}

// List returns alert groups matching a filter.
func (s *Service) List(ctx context.Context, filter contract.AlertGroupFilter) ([]contract.AlertGroup, error) {
	if filter.Severity != "" {
		validSeverities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true, "info": true, "warning": true}
		if !validSeverities[filter.Severity] {
			return nil, fmt.Errorf("alertgroup: list: invalid severity %q", filter.Severity)
		}
	}

	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	items, err := s.alertRepo.List(ctx, store.AlertGroupFilter{
		Severity:  filter.Severity,
		StartTime: filter.StartTime,
		EndTime:   filter.EndTime,
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("alertgroup: list: %w", err)
	}

	result := make([]contract.AlertGroup, 0, len(items))
	for i := range items {
		result = append(result, storeToContract(&items[i]))
	}
	return result, nil
}

// Get returns a single alert group by ID.
func (s *Service) Get(ctx context.Context, id int64) (*contract.AlertGroup, error) {
	ag, err := s.alertRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("alertgroup: get: %w", err)
	}
	out := storeToContract(ag)
	return &out, nil
}

// Escalate converts an alert group into an incident.
func (s *Service) Escalate(ctx context.Context, id int64) (*contract.EscalateResponse, error) {
	group, err := s.alertRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("alertgroup: escalate: get: %w", err)
	}
	if group.IncidentID != nil {
		return nil, fmt.Errorf("alertgroup: already escalated")
	}

	// Extract service name from labels, fallback to "unknown".
	serviceName := "unknown"
	var labels map[string]string
	if group.Labels != "" {
		_ = json.Unmarshal([]byte(group.Labels), &labels)
	}
	if v, ok := labels["service"]; ok {
		serviceName = v
	} else if v, ok := labels["service_name"]; ok {
		serviceName = v
	}

	inc, err := s.incidentSvc.CreateIncident(ctx, contract.CreateIncidentRequest{
		Source:   "alertgroup",
		Service:  serviceName,
		Severity: group.Severity,
	})
	if err != nil {
		return nil, fmt.Errorf("alertgroup: escalate: create incident: %w", err)
	}

	group.IncidentID = &inc.IncidentID
	if err := s.alertRepo.Update(ctx, group); err != nil {
		return nil, fmt.Errorf("alertgroup: escalate: update: %w", err)
	}

	s.logger.Info("alert group escalated to incident", "alert_group_id", id, "incident_id", inc.IncidentID)
	return &contract.EscalateResponse{
		AlertGroupID: id,
		IncidentID:   inc.IncidentID,
	}, nil
}

func computeFingerprint(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		_, _ = fmt.Fprintf(h, "%s=%s\n", k, labels[k])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func storeToContract(ag *store.AlertGroup) contract.AlertGroup {
	var labels map[string]string
	if ag.Labels != "" {
		_ = json.Unmarshal([]byte(ag.Labels), &labels)
	}
	return contract.AlertGroup{
		ID:          ag.ID,
		Fingerprint: ag.Fingerprint,
		Title:       ag.Title,
		Severity:    ag.Severity,
		Labels:      labels,
		IncidentID:  ag.IncidentID,
		Count:       ag.Count,
		FirstSeen:   ag.FirstSeen,
		LastSeen:    ag.LastSeen,
		CreatedAt:   ag.CreatedAt,
	}
}
