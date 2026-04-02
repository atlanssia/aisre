package tool

import (
	"context"

	"github.com/atlanssia/aisre/internal/contract"
)

// Tool defines the interface for a single analysis tool.
// Each tool orchestrates one or more adapter calls and produces a ToolResult.
type Tool interface {
	// Name returns the tool identifier.
	Name() string

	// Execute runs the tool against the given incident context.
	Execute(ctx context.Context, incident *contract.Incident) (*contract.ToolResult, error)
}
