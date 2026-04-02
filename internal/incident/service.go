package incident

import (
	"context"
	"fmt"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

type Service struct {
	repo store.IncidentRepo
}

func NewService(repo store.IncidentRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateIncident(ctx context.Context, req contract.CreateIncidentRequest) (*contract.CreateIncidentResponse, error) {
	if !contract.ValidSeverities[req.Severity] {
		return nil, fmt.Errorf("incident: invalid severity: %s", req.Severity)
	}
	if req.Service == "" {
		return nil, fmt.Errorf("incident: service is required")
	}

	inc := &store.Incident{
		Source:      req.Source,
		ServiceName: req.Service,
		Severity:    req.Severity,
		Status:      "open",
		TraceID:     req.TraceID,
	}
	id, err := s.repo.Create(ctx, inc)
	if err != nil {
		return nil, fmt.Errorf("incident: create: %w", err)
	}

	return &contract.CreateIncidentResponse{
		IncidentID: id,
		Status:     "open",
	}, nil
}

func (s *Service) GetIncident(ctx context.Context, id int64) (*contract.Incident, error) {
	inc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("incident: get: %w", err)
	}
	return &contract.Incident{
		ID:          inc.ID,
		Source:      inc.Source,
		ServiceName: inc.ServiceName,
		Severity:    inc.Severity,
		Status:      inc.Status,
		TraceID:     inc.TraceID,
		CreatedAt:   inc.CreatedAt,
	}, nil
}

func (s *Service) ListIncidents(ctx context.Context, filter store.IncidentFilter) ([]contract.Incident, error) {
	items, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("incident: list: %w", err)
	}
	result := make([]contract.Incident, len(items))
	for i, inc := range items {
		result[i] = contract.Incident{
			ID:          inc.ID,
			Source:      inc.Source,
			ServiceName: inc.ServiceName,
			Severity:    inc.Severity,
			Status:      inc.Status,
			TraceID:     inc.TraceID,
			CreatedAt:   inc.CreatedAt,
		}
	}
	return result, nil
}

func (s *Service) ProcessWebhook(ctx context.Context, payload contract.WebhookPayload) (*contract.CreateIncidentResponse, error) {
	if !contract.ValidSeverities[payload.Severity] {
		return nil, fmt.Errorf("incident: invalid severity: %s", payload.Severity)
	}
	if payload.Service == "" {
		return nil, fmt.Errorf("incident: service is required in webhook payload")
	}

	inc := &store.Incident{
		Source:      payload.Source,
		ServiceName: payload.Service,
		Severity:    payload.Severity,
		Status:      "open",
		TraceID:     payload.TraceID,
	}
	id, err := s.repo.Create(ctx, inc)
	if err != nil {
		return nil, fmt.Errorf("incident: webhook create: %w", err)
	}

	return &contract.CreateIncidentResponse{
		IncidentID: id,
		Status:     "open",
	}, nil
}
