package adapter

import "context"

// ChangeProvider fetches change events from observability backends.
// This is a Phase 2 interface, separate from ToolProvider (Go interface composition).
type ChangeProvider interface {
	GetChanges(ctx context.Context, service string, startTime, endTime string) ([]ChangeEvent, error)
}

// ChangeEvent is the adapter-level change event type.
type ChangeEvent struct {
	Service    string
	ChangeType string
	Summary    string
	Author     string
	Timestamp  string
	Metadata   map[string]any
}
