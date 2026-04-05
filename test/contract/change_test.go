package contract_test

import (
	"encoding/json"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
)

func TestChangeEvent_JSONRoundTrip(t *testing.T) {
	evt := contract.ChangeEvent{
		ID:         1,
		Service:    "api-gateway",
		ChangeType: "deploy",
		Summary:    "Deploy v3.2.1",
		Author:     "ci-bot",
		Timestamp:  "2025-01-15T09:30:00Z",
		Metadata:   map[string]any{"version": "3.2.1"},
	}
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	var got contract.ChangeEvent
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 1 || got.Service != "api-gateway" || got.ChangeType != "deploy" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestChangeQuery_JSONFields(t *testing.T) {
	q := contract.ChangeQuery{Service: "api-gw", Limit: 10}
	b, _ := json.Marshal(q)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["service"] != "api-gw" {
		t.Errorf("service: got %v", m["service"])
	}
	if m["limit"] != float64(10) {
		t.Errorf("limit: got %v", m["limit"])
	}
}

func TestChangeCorrelation_JSONRoundTrip(t *testing.T) {
	cc := contract.ChangeCorrelation{
		IncidentID: 42,
		Changes: []contract.ChangeEvent{
			{ID: 1, Service: "api-gw", ChangeType: "deploy", Summary: "Deploy v3.2.1"},
		},
		Score: 0.85,
	}
	b, _ := json.Marshal(cc)
	var got contract.ChangeCorrelation
	_ = json.Unmarshal(b, &got)
	if got.IncidentID != 42 || got.Score != 0.85 || len(got.Changes) != 1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}
