package adapter

import (
	"context"

	"github.com/atlanssia/aisre/internal/contract"
)

// ChangeProvider fetches change events from observability backends.
// This is a Phase 2 interface, separate from ToolProvider (Go interface composition).
// The consuming interface is defined in internal/change/ per CLAUDE.md convention.
type ChangeProvider interface {
	GetChanges(ctx context.Context, q contract.ChangeQuery) ([]contract.ChangeEvent, error)
}
