package analysis

import (
	"context"

	"github.com/atlanssia/aisre/internal/contract"
)

// Service defines the core analysis engine interface.
type Service interface {
	// AnalyzeIncident runs the full RCA pipeline for a given incident.
	AnalyzeIncident(ctx context.Context, incidentID int64) (*contract.RCAReport, error)
}
