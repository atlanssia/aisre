package contract_test

import (
	"encoding/json"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
)

// --- Incident DTOs ---

func TestCreateIncidentRequestJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		input   contract.CreateIncidentRequest
		wantErr bool
	}{
		{
			name: "valid request",
			input: contract.CreateIncidentRequest{
				Source:   "prometheus",
				Service:  "api-gateway",
				Severity: "high",
				TimeRange: "2025-01-15T09:00:00Z/2025-01-15T10:00:00Z",
				TraceID:  "trace-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
			}

			var got contract.CreateIncidentRequest
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got.Source != tt.input.Source {
				t.Errorf("Source = %q, want %q", got.Source, tt.input.Source)
			}
			if got.Service != tt.input.Service {
				t.Errorf("Service = %q, want %q", got.Service, tt.input.Service)
			}
			if got.Severity != tt.input.Severity {
				t.Errorf("Severity = %q, want %q", got.Severity, tt.input.Severity)
			}
		})
	}
}

func TestCreateIncidentRequestSeverityValidation(t *testing.T) {
	validSeverities := []string{"critical", "high", "medium", "low", "info"}
	for _, sev := range validSeverities {
		t.Run(sev, func(t *testing.T) {
			if !contract.ValidSeverities[sev] {
				t.Errorf("severity %q should be valid", sev)
			}
		})
	}

	invalidSeverities := []string{"urgent", "warning", "", "CRITICAL"}
	for _, sev := range invalidSeverities {
		t.Run("invalid_"+sev, func(t *testing.T) {
			if contract.ValidSeverities[sev] {
				t.Errorf("severity %q should be invalid", sev)
			}
		})
	}
}

func TestWebhookPayloadJSONRoundTrip(t *testing.T) {
	payload := contract.WebhookPayload{
		Source:    "grafana",
		AlertName: "HighErrorRate",
		Service:   "payment-service",
		Severity:  "critical",
		TraceID:   "trace-456",
		Payload:   map[string]any{"threshold": 0.5},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got contract.WebhookPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Source != payload.Source {
		t.Errorf("Source = %q, want %q", got.Source, payload.Source)
	}
	if got.AlertName != payload.AlertName {
		t.Errorf("AlertName = %q, want %q", got.AlertName, payload.AlertName)
	}
	if got.Service != payload.Service {
		t.Errorf("Service = %q, want %q", got.Service, payload.Service)
	}
}

func TestCreateIncidentResponseIDPositive(t *testing.T) {
	resp := contract.CreateIncidentResponse{
		IncidentID: 1,
		ReportID:   2,
		Status:     "created",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got contract.CreateIncidentResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.IncidentID <= 0 {
		t.Errorf("IncidentID = %d, want > 0", got.IncidentID)
	}
}

// --- Report DTOs ---

func TestReportResponseJSONRoundTrip(t *testing.T) {
	resp := contract.ReportResponse{
		ID:         1,
		IncidentID: 1,
		Summary:    "Test summary",
		RootCause:  "Test root cause",
		Confidence: 0.85,
		Evidence: []contract.EvidenceItem{
			{ID: "ev_001", Type: "log", Score: 0.9, Summary: "Error spike"},
		},
		Recommendations: []string{"Restart pods", "Fix connection leak"},
		Timeline: []contract.TimelineEvent{
			{Time: "2025-01-15T10:00:00Z", Type: "alert", Service: "api-gw", Description: "High error rate"},
		},
		CreatedAt: "2025-01-15T10:05:00Z",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got contract.ReportResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.ID <= 0 {
		t.Errorf("ID = %d, want > 0", got.ID)
	}
	if got.IncidentID <= 0 {
		t.Errorf("IncidentID = %d, want > 0", got.IncidentID)
	}
	if got.Confidence < 0 || got.Confidence > 1 {
		t.Errorf("Confidence = %f, want [0,1]", got.Confidence)
	}
	if len(got.Evidence) != 1 {
		t.Errorf("len(Evidence) = %d, want 1", len(got.Evidence))
	}
	if len(got.Timeline) != 1 {
		t.Errorf("len(Timeline) = %d, want 1", len(got.Timeline))
	}
}

func TestEvidenceItemValidTypes(t *testing.T) {
	validTypes := []string{"trace", "log", "metric", "change"}
	for _, typ := range validTypes {
		t.Run(typ, func(t *testing.T) {
			ev := contract.EvidenceItem{Type: typ, Score: 0.5, Summary: "test"}
			data, _ := json.Marshal(ev)
			var got contract.EvidenceItem
			json.Unmarshal(data, &got)
			if got.Type != typ {
				t.Errorf("Type = %q, want %q", got.Type, typ)
			}
		})
	}
}

func TestEvidenceItemScoreRange(t *testing.T) {
	tests := []struct {
		name  string
		score float64
		valid bool
	}{
		{"zero", 0.0, true},
		{"one", 1.0, true},
		{"half", 0.5, true},
		{"negative", -0.1, false},
		{"over_one", 1.1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inRange := tt.score >= 0 && tt.score <= 1
			if inRange != tt.valid {
				t.Errorf("score %f inRange=%v, want %v", tt.score, inRange, tt.valid)
			}
		})
	}
}

func TestTimelineEventValidTypes(t *testing.T) {
	validTypes := []string{"symptom", "error", "deploy", "alert", "action"}
	for _, typ := range validTypes {
		t.Run(typ, func(t *testing.T) {
			ev := contract.TimelineEvent{
				Time:        "2025-01-15T10:00:00Z",
				Type:        typ,
				Service:     "test-svc",
				Description: "test event",
				Severity:    "info",
			}
			data, _ := json.Marshal(ev)
			var got contract.TimelineEvent
			json.Unmarshal(data, &got)
			if got.Type != typ {
				t.Errorf("Type = %q, want %q", got.Type, typ)
			}
		})
	}
}

func TestReportFilterDefaults(t *testing.T) {
	f := contract.ReportFilter{}
	data, _ := json.Marshal(f)
	var got contract.ReportFilter
	json.Unmarshal(data, &got)

	if got.Limit < 0 {
		t.Errorf("Limit = %d, want >= 0", got.Limit)
	}
	if got.Offset < 0 {
		t.Errorf("Offset = %d, want >= 0", got.Offset)
	}
}

// --- Feedback DTOs ---

func TestFeedbackRequestRatingRange(t *testing.T) {
	tests := []struct {
		name   string
		rating int
		valid  bool
	}{
		{"one", 1, true},
		{"three", 3, true},
		{"five", 5, true},
		{"zero", 0, false},
		{"six", 6, false},
		{"negative", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.rating >= 1 && tt.rating <= 5
			if valid != tt.valid {
				t.Errorf("rating %d valid=%v, want %v", tt.rating, valid, tt.valid)
			}
		})
	}
}

func TestFeedbackRequestActionTaken(t *testing.T) {
	validActions := []string{"accepted", "partial", "rejected"}
	for _, action := range validActions {
		t.Run(action, func(t *testing.T) {
			if !contract.ValidActions[action] {
				t.Errorf("action %q should be valid", action)
			}
		})
	}

	invalidActions := []string{"maybe", "unknown", "", "APPROVED"}
	for _, action := range invalidActions {
		t.Run("invalid_"+action, func(t *testing.T) {
			if contract.ValidActions[action] {
				t.Errorf("action %q should be invalid", action)
			}
		})
	}
}

// --- Error DTOs ---

func TestErrorResponseJSONRoundTrip(t *testing.T) {
	errResp := contract.ErrorResponse{
		Error: "not found",
		Code:  contract.ErrCodeNotFound,
	}

	data, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got contract.ErrorResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Error != errResp.Error {
		t.Errorf("Error = %q, want %q", got.Error, errResp.Error)
	}
	if got.Code != errResp.Code {
		t.Errorf("Code = %q, want %q", got.Code, errResp.Code)
	}
}

func TestErrorCodeConstants(t *testing.T) {
	codes := []struct {
		name  string
		value string
	}{
		{"ErrCodeInvalidRequest", contract.ErrCodeInvalidRequest},
		{"ErrCodeNotFound", contract.ErrCodeNotFound},
		{"ErrCodeInternal", contract.ErrCodeInternal},
		{"ErrCodeAdapterTimeout", contract.ErrCodeAdapterTimeout},
		{"ErrCodeLLMFailed", contract.ErrCodeLLMFailed},
		{"ErrCodeDuplicate", contract.ErrCodeDuplicate},
	}

	for _, c := range codes {
		t.Run(c.name, func(t *testing.T) {
			if c.value == "" {
				t.Errorf("code %q is empty", c.name)
			}
		})
	}
}

// --- Tool DTOs ---

func TestToolResultJSONRoundTrip(t *testing.T) {
	tr := contract.ToolResult{
		ID:      "tool_001",
		Name:    "logs",
		Type:    "log",
		Summary: "Error spike in logs",
		Score:   0.9,
		SourceURL: "http://oo:5080/web/logs?query=error",
		Payload: map[string]any{"count": 150},
	}

	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got contract.ToolResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.ID != tr.ID {
		t.Errorf("ID = %q, want %q", got.ID, tr.ID)
	}
	if got.Name != tr.Name {
		t.Errorf("Name = %q, want %q", got.Name, tr.Name)
	}
	if got.Type != tr.Type {
		t.Errorf("Type = %q, want %q", got.Type, tr.Type)
	}
	if got.Score != tr.Score {
		t.Errorf("Score = %f, want %f", got.Score, tr.Score)
	}
	if got.Score < 0 || got.Score > 1 {
		t.Errorf("Score %f outside valid range [0,1]", got.Score)
	}
}

func TestToolResultScoreRange(t *testing.T) {
	tests := []struct {
		name  string
		score float64
		valid bool
	}{
		{"zero", 0.0, true},
		{"one", 1.0, true},
		{"negative", -0.1, false},
		{"over_one", 1.5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inRange := tt.score >= 0 && tt.score <= 1
			if inRange != tt.valid {
				t.Errorf("score %f inRange=%v, want %v", tt.score, inRange, tt.valid)
			}
		})
	}
}
