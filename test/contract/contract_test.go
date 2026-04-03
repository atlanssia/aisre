package contract_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
)

// --- helpers ---

// assertRoundTrip marshals v to JSON, unmarshals back into a new value of the same type,
// then re-marshals and compares the two JSON outputs. This verifies that JSON tags are
// correct and no data is lost during serialization/deserialization.
func assertRoundTrip(t *testing.T, v any) {
	t.Helper()

	original, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal %T: %v", v, err)
	}

	// Create a new zero value of the same type and unmarshal into it.
	rv := reflect.New(reflect.TypeOf(v))
	if err := json.Unmarshal(original, rv.Interface()); err != nil {
		t.Fatalf("failed to unmarshal %T JSON: %v", v, err)
	}
	roundTripped, err := json.Marshal(rv.Interface())
	if err != nil {
		t.Fatalf("failed to re-marshal %T: %v", v, err)
	}

	if string(original) != string(roundTripped) {
		t.Errorf("JSON round-trip mismatch for %T:\noriginal:     %s\nround-tripped: %s", v, original, roundTripped)
	}
}

// mustMarshal returns the JSON bytes or fatals the test.
func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal %T: %v", v, err)
	}
	return b
}

// mustUnmarshalMap unmarshals JSON bytes into a map or fatals the test.
func mustUnmarshalMap(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal JSON into map: %v", err)
	}
	return m
}

// ========================================================================
// Incident DTOs
// ========================================================================

func TestCreateIncidentRequest_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		req      contract.CreateIncidentRequest
		wantJSON string
	}{
		{
			name: "all fields",
			req: contract.CreateIncidentRequest{
				Source:    "prometheus",
				Service:   "payment-service",
				Severity:  "critical",
				TimeRange: "last_15m",
				TraceID:   "abc-123",
			},
			wantJSON: `{"source":"prometheus","service":"payment-service","severity":"critical","time_range":"last_15m","trace_id":"abc-123"}`,
		},
		{
			name: "without optional trace_id",
			req: contract.CreateIncidentRequest{
				Source:    "grafana",
				Service:   "auth-service",
				Severity:  "high",
				TimeRange: "last_5m",
			},
			wantJSON: `{"source":"grafana","service":"auth-service","severity":"high","time_range":"last_5m"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(mustMarshal(t, tt.req))
			if got != tt.wantJSON {
				t.Errorf("Marshal() = %s, want %s", got, tt.wantJSON)
			}
			assertRoundTrip(t, tt.req)
		})
	}
}

func TestCreateIncidentRequest_RequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		req     contract.CreateIncidentRequest
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing source",
			req:     contract.CreateIncidentRequest{Service: "svc", Severity: "high", TimeRange: "5m"},
			wantErr: true,
			errMsg:  "source is required",
		},
		{
			name:    "missing service",
			req:     contract.CreateIncidentRequest{Source: "prom", Severity: "high", TimeRange: "5m"},
			wantErr: true,
			errMsg:  "service is required",
		},
		{
			name:    "missing severity",
			req:     contract.CreateIncidentRequest{Source: "prom", Service: "svc", TimeRange: "5m"},
			wantErr: true,
			errMsg:  "severity is required",
		},
		{
			name:    "invalid severity",
			req:     contract.CreateIncidentRequest{Source: "prom", Service: "svc", Severity: "urgent", TimeRange: "5m"},
			wantErr: true,
			errMsg:  "invalid severity",
		},
		{
			name:    "valid request",
			req:     contract.CreateIncidentRequest{Source: "prom", Service: "svc", Severity: "critical", TimeRange: "5m"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateIncidentRequest(tt.req)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("error = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestCreateIncidentResponse_JSONRoundTrip(t *testing.T) {
	resp := contract.CreateIncidentResponse{
		IncidentID: 42,
		ReportID:   7,
		Status:     "analyzing",
	}
	assertRoundTrip(t, resp)

	data := mustMarshal(t, resp)
	m := mustUnmarshalMap(t, data)

	if incidentID, ok := m["incident_id"].(float64); !ok || incidentID <= 0 {
		t.Errorf("incident_id must be > 0, got %v", m["incident_id"])
	}
}

func TestCreateIncidentResponse_IncidentIDPositive(t *testing.T) {
	tests := []struct {
		name      string
		resp      contract.CreateIncidentResponse
		wantValid bool
	}{
		{
			name:      "positive incident_id",
			resp:      contract.CreateIncidentResponse{IncidentID: 1, ReportID: 1, Status: "analyzing"},
			wantValid: true,
		},
		{
			name:      "zero incident_id is invalid",
			resp:      contract.CreateIncidentResponse{IncidentID: 0, ReportID: 1, Status: "analyzing"},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRoundTrip(t, tt.resp)
			if tt.resp.IncidentID <= 0 && tt.wantValid {
				t.Error("incident_id must be > 0")
			}
			if tt.resp.IncidentID > 0 && !tt.wantValid {
				t.Error("expected incident_id to be invalid (<= 0)")
			}
		})
	}
}

func TestWebhookPayload_JSONRoundTrip(t *testing.T) {
	wp := contract.WebhookPayload{
		Source:    "prometheus",
		AlertName: "HighErrorRate",
		Service:   "api-gateway",
		Severity:  "critical",
		TraceID:   "trace-456",
		Payload:   map[string]any{"firing": true, "threshold": 0.05},
	}
	assertRoundTrip(t, wp)

	data := mustMarshal(t, wp)
	m := mustUnmarshalMap(t, data)

	// Verify required fields are present in JSON output
	for _, key := range []string{"source", "alert_name", "service", "severity"} {
		if _, ok := m[key]; !ok {
			t.Errorf("required field %q missing from JSON output", key)
		}
	}
}

func TestWebhookPayload_RequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		wp      contract.WebhookPayload
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing source",
			wp:      contract.WebhookPayload{AlertName: "alert", Service: "svc", Severity: "high"},
			wantErr: true,
			errMsg:  "source is required",
		},
		{
			name:    "missing alert_name",
			wp:      contract.WebhookPayload{Source: "prom", Service: "svc", Severity: "high"},
			wantErr: true,
			errMsg:  "alert_name is required",
		},
		{
			name:    "missing service",
			wp:      contract.WebhookPayload{Source: "prom", AlertName: "alert", Severity: "high"},
			wantErr: true,
			errMsg:  "service is required",
		},
		{
			name:    "missing severity",
			wp:      contract.WebhookPayload{Source: "prom", AlertName: "alert", Service: "svc"},
			wantErr: true,
			errMsg:  "severity is required",
		},
		{
			name:    "invalid severity",
			wp:      contract.WebhookPayload{Source: "prom", AlertName: "alert", Service: "svc", Severity: "extreme"},
			wantErr: true,
			errMsg:  "invalid severity",
		},
		{
			name:    "valid payload",
			wp:      contract.WebhookPayload{Source: "prom", AlertName: "alert", Service: "svc", Severity: "critical"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWebhookPayload(tt.wp)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("error = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestIncident_JSONRoundTrip(t *testing.T) {
	inc := contract.Incident{
		ID:          1,
		Source:      "prometheus",
		ServiceName: "payment-service",
		Severity:    "critical",
		Status:      "open",
		CreatedAt:   "2025-04-01T12:00:00Z",
	}
	assertRoundTrip(t, inc)

	data := mustMarshal(t, inc)
	m := mustUnmarshalMap(t, data)

	// Verify json tag mapping
	if _, ok := m["service_name"]; !ok {
		t.Error("expected 'service_name' key in JSON output")
	}
	if _, ok := m["trace_id"]; ok {
		t.Error("omitempty trace_id should not appear when empty")
	}
}

// ========================================================================
// Report DTOs
// ========================================================================

func TestReportResponse_JSONRoundTrip(t *testing.T) {
	resp := contract.ReportResponse{
		ID:         1,
		IncidentID: 42,
		Summary:    "Database connection pool exhaustion",
		RootCause:  "Max connections set too low for traffic volume",
		Confidence: 0.85,
		Evidence: []contract.EvidenceItem{
			{ID: "ev1", Type: "log", Score: 0.9, Summary: "Connection refused errors"},
		},
		Recommendations: []string{"Increase max connections", "Add connection pooling"},
		Timeline: []contract.TimelineEvent{
			{Time: "12:00:00", Type: "alert", Service: "db", Description: "Alert fired"},
		},
		CreatedAt: "2025-04-01T12:05:00Z",
	}
	assertRoundTrip(t, resp)
}

func TestReportResponse_Constraints(t *testing.T) {
	tests := []struct {
		name      string
		resp      contract.ReportResponse
		wantValid bool
	}{
		{
			name:      "valid report",
			resp:      contract.ReportResponse{ID: 1, IncidentID: 1, Confidence: 0.5},
			wantValid: true,
		},
		{
			name:      "zero id is invalid",
			resp:      contract.ReportResponse{ID: 0, IncidentID: 1, Confidence: 0.5},
			wantValid: false,
		},
		{
			name:      "zero incident_id is invalid",
			resp:      contract.ReportResponse{ID: 1, IncidentID: 0, Confidence: 0.5},
			wantValid: false,
		},
		{
			name:      "confidence at lower bound",
			resp:      contract.ReportResponse{ID: 1, IncidentID: 1, Confidence: 0.0},
			wantValid: true,
		},
		{
			name:      "confidence at upper bound",
			resp:      contract.ReportResponse{ID: 1, IncidentID: 1, Confidence: 1.0},
			wantValid: true,
		},
		{
			name:      "confidence below zero is invalid",
			resp:      contract.ReportResponse{ID: 1, IncidentID: 1, Confidence: -0.1},
			wantValid: false,
		},
		{
			name:      "confidence above one is invalid",
			resp:      contract.ReportResponse{ID: 1, IncidentID: 1, Confidence: 1.5},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReportResponse(tt.resp)
			if tt.wantValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.wantValid && err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestEvidenceItem_JSONRoundTrip(t *testing.T) {
	ev := contract.EvidenceItem{
		ID:        "ev-001",
		Type:      "trace",
		Score:     0.92,
		Summary:   "Slow database query detected",
		SourceURL: "http://jaeger:16686/trace/abc",
		Payload:   map[string]any{"duration_ms": 5000},
	}
	assertRoundTrip(t, ev)
}

func TestEvidenceItem_Constraints(t *testing.T) {
	validTypes := map[string]bool{"trace": true, "log": true, "metric": true, "change": true}

	tests := []struct {
		name      string
		ev        contract.EvidenceItem
		wantValid bool
	}{
		{
			name:      "valid trace evidence",
			ev:        contract.EvidenceItem{ID: "ev1", Type: "trace", Score: 0.8, Summary: "test"},
			wantValid: true,
		},
		{
			name:      "valid log evidence",
			ev:        contract.EvidenceItem{ID: "ev1", Type: "log", Score: 0.5, Summary: "test"},
			wantValid: true,
		},
		{
			name:      "valid metric evidence",
			ev:        contract.EvidenceItem{ID: "ev1", Type: "metric", Score: 0.3, Summary: "test"},
			wantValid: true,
		},
		{
			name:      "valid change evidence",
			ev:        contract.EvidenceItem{ID: "ev1", Type: "change", Score: 0.7, Summary: "test"},
			wantValid: true,
		},
		{
			name:      "invalid type",
			ev:        contract.EvidenceItem{ID: "ev1", Type: "unknown", Score: 0.5, Summary: "test"},
			wantValid: false,
		},
		{
			name:      "score below zero",
			ev:        contract.EvidenceItem{ID: "ev1", Type: "log", Score: -0.1, Summary: "test"},
			wantValid: false,
		},
		{
			name:      "score above one",
			ev:        contract.EvidenceItem{ID: "ev1", Type: "log", Score: 1.5, Summary: "test"},
			wantValid: false,
		},
		{
			name:      "score at lower bound",
			ev:        contract.EvidenceItem{ID: "ev1", Type: "log", Score: 0.0, Summary: "test"},
			wantValid: true,
		},
		{
			name:      "score at upper bound",
			ev:        contract.EvidenceItem{ID: "ev1", Type: "log", Score: 1.0, Summary: "test"},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEvidenceItem(tt.ev, validTypes)
			if tt.wantValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.wantValid && err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestTimelineEvent_JSONRoundTrip(t *testing.T) {
	evt := contract.TimelineEvent{
		Time:        "12:00:00",
		Type:        "error",
		Service:     "payment-service",
		Description: "Connection timeout to database",
		Severity:    "high",
	}
	assertRoundTrip(t, evt)

	// Test omitempty severity
	evtNoSev := contract.TimelineEvent{
		Time:        "12:00:00",
		Type:        "deploy",
		Service:     "api-gateway",
		Description: "Deployed v2.1.0",
	}
	data := mustMarshal(t, evtNoSev)
	m := mustUnmarshalMap(t, data)
	if _, ok := m["severity"]; ok {
		t.Error("omitempty severity should not appear when empty")
	}
}

func TestTimelineEvent_Constraints(t *testing.T) {
	validTypes := map[string]bool{
		"symptom": true, "error": true, "deploy": true, "alert": true, "action": true,
	}

	tests := []struct {
		name      string
		evt       contract.TimelineEvent
		wantValid bool
	}{
		{
			name:      "valid symptom",
			evt:       contract.TimelineEvent{Time: "12:00", Type: "symptom", Service: "svc", Description: "Slow response"},
			wantValid: true,
		},
		{
			name:      "valid error",
			evt:       contract.TimelineEvent{Time: "12:00", Type: "error", Service: "svc", Description: "Error"},
			wantValid: true,
		},
		{
			name:      "valid deploy",
			evt:       contract.TimelineEvent{Time: "12:00", Type: "deploy", Service: "svc", Description: "Deploy"},
			wantValid: true,
		},
		{
			name:      "valid alert",
			evt:       contract.TimelineEvent{Time: "12:00", Type: "alert", Service: "svc", Description: "Alert"},
			wantValid: true,
		},
		{
			name:      "valid action",
			evt:       contract.TimelineEvent{Time: "12:00", Type: "action", Service: "svc", Description: "Action"},
			wantValid: true,
		},
		{
			name:      "invalid type",
			evt:       contract.TimelineEvent{Time: "12:00", Type: "unknown", Service: "svc", Description: "Unknown"},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimelineEvent(tt.evt, validTypes)
			if tt.wantValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.wantValid && err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestReportFilter_JSONRoundTrip(t *testing.T) {
	rf := contract.ReportFilter{
		Service:   "payment-service",
		StartTime: "2025-04-01T00:00:00Z",
		EndTime:   "2025-04-01T23:59:59Z",
		Severity:  "critical",
		Limit:     20,
		Offset:    10,
	}
	assertRoundTrip(t, rf)

	// Test omitempty fields
	rfEmpty := contract.ReportFilter{}
	data := mustMarshal(t, rfEmpty)
	m := mustUnmarshalMap(t, data)
	for _, key := range []string{"service", "start_time", "end_time", "severity", "limit", "offset"} {
		if _, ok := m[key]; ok {
			t.Errorf("omitempty field %q should not appear when zero-valued", key)
		}
	}
}

func TestReportFilter_Constraints(t *testing.T) {
	tests := []struct {
		name      string
		rf        contract.ReportFilter
		wantValid bool
	}{
		{
			name:      "valid filter with limit and offset",
			rf:        contract.ReportFilter{Limit: 10, Offset: 0},
			wantValid: true,
		},
		{
			name:      "limit zero with non-zero offset is invalid",
			rf:        contract.ReportFilter{Limit: 0, Offset: 10},
			wantValid: false,
		},
		{
			name:      "negative limit is invalid",
			rf:        contract.ReportFilter{Limit: -1, Offset: 0},
			wantValid: false,
		},
		{
			name:      "negative offset is invalid",
			rf:        contract.ReportFilter{Limit: 10, Offset: -1},
			wantValid: false,
		},
		{
			name:      "empty filter is valid",
			rf:        contract.ReportFilter{},
			wantValid: true,
		},
		{
			name:      "limit set with zero offset is valid",
			rf:        contract.ReportFilter{Limit: 5, Offset: 0},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReportFilter(tt.rf)
			if tt.wantValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.wantValid && err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

// ========================================================================
// Feedback DTOs
// ========================================================================

func TestFeedbackRequest_JSONRoundTrip(t *testing.T) {
	fr := contract.FeedbackRequest{
		Rating:      4,
		Comment:     "Helpful analysis but missed one key log",
		UserID:      "user-42",
		ActionTaken: "partial",
	}
	assertRoundTrip(t, fr)
}

func TestFeedbackRequest_Constraints(t *testing.T) {
	tests := []struct {
		name      string
		fr        contract.FeedbackRequest
		wantValid bool
	}{
		{
			name:      "rating 1 accepted",
			fr:        contract.FeedbackRequest{Rating: 1, ActionTaken: "rejected"},
			wantValid: true,
		},
		{
			name:      "rating 5 accepted",
			fr:        contract.FeedbackRequest{Rating: 5, ActionTaken: "accepted"},
			wantValid: true,
		},
		{
			name:      "rating 0 is invalid",
			fr:        contract.FeedbackRequest{Rating: 0, ActionTaken: "rejected"},
			wantValid: false,
		},
		{
			name:      "rating 6 is invalid",
			fr:        contract.FeedbackRequest{Rating: 6, ActionTaken: "accepted"},
			wantValid: false,
		},
		{
			name:      "rating -1 is invalid",
			fr:        contract.FeedbackRequest{Rating: -1, ActionTaken: "rejected"},
			wantValid: false,
		},
		{
			name:      "action accepted is valid",
			fr:        contract.FeedbackRequest{Rating: 3, ActionTaken: "accepted"},
			wantValid: true,
		},
		{
			name:      "action partial is valid",
			fr:        contract.FeedbackRequest{Rating: 3, ActionTaken: "partial"},
			wantValid: true,
		},
		{
			name:      "action rejected is valid",
			fr:        contract.FeedbackRequest{Rating: 3, ActionTaken: "rejected"},
			wantValid: true,
		},
		{
			name:      "action unknown is invalid",
			fr:        contract.FeedbackRequest{Rating: 3, ActionTaken: "maybe"},
			wantValid: false,
		},
		{
			name:      "empty action_taken is invalid",
			fr:        contract.FeedbackRequest{Rating: 3, ActionTaken: ""},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFeedbackRequest(tt.fr)
			if tt.wantValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.wantValid && err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestFeedbackResponse_JSONRoundTrip(t *testing.T) {
	fr := contract.FeedbackResponse{
		ID:          1,
		ReportID:    42,
		Rating:      5,
		Comment:     "Spot on analysis",
		UserID:      "user-1",
		ActionTaken: "accepted",
		CreatedAt:   "2025-04-01T12:30:00Z",
	}
	assertRoundTrip(t, fr)
}

func TestSearchReportsRequest_JSONRoundTrip(t *testing.T) {
	sr := contract.SearchReportsRequest{
		Query:    "database timeout",
		Service:  "payment-service",
		Severity: "high",
		Limit:    10,
		Offset:   0,
	}
	assertRoundTrip(t, sr)

	// Test omitempty
	srEmpty := contract.SearchReportsRequest{}
	data := mustMarshal(t, srEmpty)
	m := mustUnmarshalMap(t, data)
	for _, key := range []string{"query", "service", "severity", "limit", "offset"} {
		if _, ok := m[key]; ok {
			t.Errorf("omitempty field %q should not appear when zero-valued", key)
		}
	}
}

// ========================================================================
// Error DTOs
// ========================================================================

func TestErrorResponse_JSONRoundTrip(t *testing.T) {
	errResp := contract.ErrorResponse{
		Error: "incident not found",
		Code:  contract.ErrCodeNotFound,
	}
	assertRoundTrip(t, errResp)

	data := mustMarshal(t, errResp)
	m := mustUnmarshalMap(t, data)

	// Verify both fields are present
	if _, ok := m["error"]; !ok {
		t.Error("missing 'error' field in ErrorResponse JSON")
	}
	if _, ok := m["code"]; !ok {
		t.Error("missing 'code' field in ErrorResponse JSON")
	}
}

func TestErrorCodeConstants(t *testing.T) {
	wantCodes := map[string]string{
		"ErrCodeInvalidRequest": "INVALID_REQUEST",
		"ErrCodeNotFound":       "NOT_FOUND",
		"ErrCodeInternal":       "INTERNAL_ERROR",
		"ErrCodeAdapterTimeout": "ADAPTER_TIMEOUT",
		"ErrCodeLLMFailed":      "LLM_FAILED",
		"ErrCodeDuplicate":      "DUPLICATE",
	}

	gotCodes := map[string]string{
		"ErrCodeInvalidRequest": contract.ErrCodeInvalidRequest,
		"ErrCodeNotFound":       contract.ErrCodeNotFound,
		"ErrCodeInternal":       contract.ErrCodeInternal,
		"ErrCodeAdapterTimeout": contract.ErrCodeAdapterTimeout,
		"ErrCodeLLMFailed":      contract.ErrCodeLLMFailed,
		"ErrCodeDuplicate":      contract.ErrCodeDuplicate,
	}

	for name, want := range wantCodes {
		got, ok := gotCodes[name]
		if !ok {
			t.Errorf("constant %s not found", name)
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestValidSeverities(t *testing.T) {
	want := map[string]bool{
		"critical": true,
		"high":     true,
		"medium":   true,
		"low":      true,
		"info":     true,
	}

	for sev := range want {
		if !contract.ValidSeverities[sev] {
			t.Errorf("ValidSeverities[%q] = false, want true", sev)
		}
	}

	// Verify invalid values are not present
	invalid := []string{"urgent", "extreme", "warning", "CRITICAL", "High"}
	for _, sev := range invalid {
		if contract.ValidSeverities[sev] {
			t.Errorf("ValidSeverities[%q] = true, want false", sev)
		}
	}
}

func TestValidActions(t *testing.T) {
	want := map[string]bool{
		"accepted": true,
		"partial":  true,
		"rejected": true,
	}

	for action := range want {
		if !contract.ValidActions[action] {
			t.Errorf("ValidActions[%q] = false, want true", action)
		}
	}

	invalid := []string{"maybe", "done", "ACCEPTED", "Partial"}
	for _, action := range invalid {
		if contract.ValidActions[action] {
			t.Errorf("ValidActions[%q] = true, want false", action)
		}
	}
}

// ========================================================================
// Tool DTOs
// ========================================================================

func TestToolResult_JSONRoundTrip(t *testing.T) {
	tr := contract.ToolResult{
		ID:        "tool-001",
		Name:      "search_logs",
		Type:      "log",
		Summary:   "Found 15 error logs in payment-service",
		Score:     0.88,
		SourceURL: "http://openobserve:5080/logs/abc",
		Payload:   map[string]any{"count": 15, "service": "payment-service"},
	}
	assertRoundTrip(t, tr)
}

func TestToolResult_Fields(t *testing.T) {
	tr := contract.ToolResult{
		ID:      "tool-001",
		Name:    "search_logs",
		Type:    "log",
		Summary: "Found errors",
		Score:   0.88,
		Payload: map[string]any{},
	}

	data := mustMarshal(t, tr)
	m := mustUnmarshalMap(t, data)

	requiredFields := []string{"id", "name", "type", "summary", "score"}
	for _, field := range requiredFields {
		if _, ok := m[field]; !ok {
			t.Errorf("required field %q missing from ToolResult JSON", field)
		}
	}
}

func TestToolResult_ScoreConstraints(t *testing.T) {
	tests := []struct {
		name      string
		tr        contract.ToolResult
		wantValid bool
	}{
		{
			name:      "score at lower bound",
			tr:        contract.ToolResult{ID: "t1", Name: "n", Type: "log", Summary: "s", Score: 0.0},
			wantValid: true,
		},
		{
			name:      "score at upper bound",
			tr:        contract.ToolResult{ID: "t1", Name: "n", Type: "log", Summary: "s", Score: 1.0},
			wantValid: true,
		},
		{
			name:      "score in range",
			tr:        contract.ToolResult{ID: "t1", Name: "n", Type: "log", Summary: "s", Score: 0.5},
			wantValid: true,
		},
		{
			name:      "score below zero is invalid",
			tr:        contract.ToolResult{ID: "t1", Name: "n", Type: "log", Summary: "s", Score: -0.1},
			wantValid: false,
		},
		{
			name:      "score above one is invalid",
			tr:        contract.ToolResult{ID: "t1", Name: "n", Type: "log", Summary: "s", Score: 1.1},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateToolResultScore(tt.tr)
			if tt.wantValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.wantValid && err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestToolResult_OmitEmptySourceURL(t *testing.T) {
	tr := contract.ToolResult{
		ID:      "tool-001",
		Name:    "search_logs",
		Type:    "log",
		Summary: "Found errors",
		Score:   0.88,
		Payload: map[string]any{},
	}
	data := mustMarshal(t, tr)
	m := mustUnmarshalMap(t, data)
	if _, ok := m["source_url"]; ok {
		t.Error("omitempty source_url should not appear when empty")
	}
}

// ========================================================================
// Validation helpers
// ========================================================================

func validateCreateIncidentRequest(req contract.CreateIncidentRequest) error {
	if req.Source == "" {
		return fmt.Errorf("source is required")
	}
	if req.Service == "" {
		return fmt.Errorf("service is required")
	}
	if req.Severity == "" {
		return fmt.Errorf("severity is required")
	}
	if !contract.ValidSeverities[req.Severity] {
		return fmt.Errorf("invalid severity")
	}
	return nil
}

func validateWebhookPayload(wp contract.WebhookPayload) error {
	if wp.Source == "" {
		return fmt.Errorf("source is required")
	}
	if wp.AlertName == "" {
		return fmt.Errorf("alert_name is required")
	}
	if wp.Service == "" {
		return fmt.Errorf("service is required")
	}
	if wp.Severity == "" {
		return fmt.Errorf("severity is required")
	}
	if !contract.ValidSeverities[wp.Severity] {
		return fmt.Errorf("invalid severity")
	}
	return nil
}

func validateReportResponse(resp contract.ReportResponse) error {
	if resp.ID <= 0 {
		return fmt.Errorf("id must be > 0")
	}
	if resp.IncidentID <= 0 {
		return fmt.Errorf("incident_id must be > 0")
	}
	if resp.Confidence < 0.0 || resp.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0")
	}
	return nil
}

func validateEvidenceItem(ev contract.EvidenceItem, validTypes map[string]bool) error {
	if !validTypes[ev.Type] {
		return fmt.Errorf("invalid evidence type: %s", ev.Type)
	}
	if ev.Score < 0.0 || ev.Score > 1.0 {
		return fmt.Errorf("score must be between 0.0 and 1.0")
	}
	return nil
}

func validateTimelineEvent(evt contract.TimelineEvent, validTypes map[string]bool) error {
	if !validTypes[evt.Type] {
		return fmt.Errorf("invalid timeline event type: %s", evt.Type)
	}
	return nil
}

func validateReportFilter(rf contract.ReportFilter) error {
	if rf.Limit < 0 {
		return fmt.Errorf("limit must be >= 0")
	}
	if rf.Offset < 0 {
		return fmt.Errorf("offset must be >= 0")
	}
	if rf.Offset > 0 && rf.Limit <= 0 {
		return fmt.Errorf("limit must be > 0 when offset is set")
	}
	return nil
}

func validateFeedbackRequest(fr contract.FeedbackRequest) error {
	if fr.Rating < 1 || fr.Rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	if !contract.ValidActions[fr.ActionTaken] {
		return fmt.Errorf("action_taken must be one of accepted, partial, rejected")
	}
	return nil
}

func validateToolResultScore(tr contract.ToolResult) error {
	if tr.Score < 0.0 || tr.Score > 1.0 {
		return fmt.Errorf("score must be between 0.0 and 1.0")
	}
	return nil
}
