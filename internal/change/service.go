package change

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// ChangeFinder fetches change events for correlation (defined in consuming package per CLAUDE.md).
type ChangeFinder interface {
	GetChangesForIncident(ctx context.Context, incidentID int64) (*contract.ChangeCorrelation, error)
}

// Service correlates change events with incidents.
type Service struct {
	changeRepo store.ChangeRepo
	incRepo    store.IncidentRepo
	rptRepo    store.ReportRepo
	logger     *slog.Logger
}

// NewService creates a new change correlation service.
func NewService(changeRepo store.ChangeRepo, incRepo store.IncidentRepo, rptRepo store.ReportRepo) *Service {
	return &Service{
		changeRepo: changeRepo,
		incRepo:    incRepo,
		rptRepo:    rptRepo,
		logger:     slog.Default(),
	}
}

// GetChanges lists change events matching the given query.
func (s *Service) GetChanges(ctx context.Context, q contract.ChangeQuery) ([]contract.ChangeEvent, error) {
	filter := store.ChangeFilter{
		Service:     q.Service,
		ChangeTypes: q.ChangeTypes,
		StartTime:   q.StartTime,
		EndTime:     q.EndTime,
		Limit:       q.Limit,
		Offset:      q.Offset,
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	results, err := s.changeRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("change: list: %w", err)
	}

	events := make([]contract.ChangeEvent, len(results))
	for i, ch := range results {
		events[i] = storeChangeToContract(ch)
	}
	return events, nil
}

// GetChangesForIncident retrieves changes correlated with an incident's time window.
func (s *Service) GetChangesForIncident(ctx context.Context, incidentID int64) (*contract.ChangeCorrelation, error) {
	inc, err := s.incRepo.GetByID(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("change: get incident: %w", err)
	}

	// Look back 2 hours before incident, up to incident time
	startTime := ""
	endTime := inc.CreatedAt
	if t, err := time.Parse("2006-01-02 15:04:05", inc.CreatedAt); err == nil {
		startTime = t.Add(-2 * time.Hour).Format(time.RFC3339)
	} else if t, err := time.Parse(time.RFC3339, inc.CreatedAt); err == nil {
		startTime = t.Add(-2 * time.Hour).Format(time.RFC3339)
	} else {
		// Fallback: no start time filter, just use incident time as end
		startTime = ""
	}

	results, err := s.changeRepo.ListByService(ctx, inc.ServiceName, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("change: list by service: %w", err)
	}

	events := make([]contract.ChangeEvent, len(results))
	for i, ch := range results {
		events[i] = storeChangeToContract(ch)
	}

	return &contract.ChangeCorrelation{
		IncidentID: incidentID,
		Changes:    events,
	}, nil
}

// IngestChange stores a new change event.
func (s *Service) IngestChange(ctx context.Context, evt contract.ChangeEvent) (int64, error) {
	metadata, _ := json.Marshal(evt.Metadata)
	ch := &store.Change{
		Service:    evt.Service,
		ChangeType: evt.ChangeType,
		Summary:    evt.Summary,
		Author:     evt.Author,
		Timestamp:  evt.Timestamp,
		Metadata:   string(metadata),
	}
	id, err := s.changeRepo.Create(ctx, ch)
	if err != nil {
		return 0, fmt.Errorf("change: ingest: %w", err)
	}
	s.logger.Info("change ingested", "id", id, "service", evt.Service, "type", evt.ChangeType)
	return id, nil
}

func storeChangeToContract(ch store.Change) contract.ChangeEvent {
	var metadata map[string]any
	if ch.Metadata != "" && ch.Metadata != "{}" {
		_ = json.Unmarshal([]byte(ch.Metadata), &metadata)
	}
	return contract.ChangeEvent{
		ID:         ch.ID,
		Service:    ch.Service,
		ChangeType: ch.ChangeType,
		Summary:    ch.Summary,
		Author:     ch.Author,
		Timestamp:  ch.Timestamp,
		Metadata:   metadata,
	}
}
