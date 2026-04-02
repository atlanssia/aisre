package openobserve

import (
	"github.com/atlanssia/aisre/internal/contract"
)

// mapLogHit maps a raw OO log hit to a ToolResult.
func mapLogHit(hit map[string]any, score float64) contract.ToolResult {
	summary := extractString(hit, "log")
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}
	return contract.ToolResult{
		Name:    "critical_log_cluster",
		Summary: summary,
		Score:   score,
		Payload: hit,
	}
}

// mapSpan maps a raw OO span to a ToolResult.
func mapSpan(span map[string]any, score float64) contract.ToolResult {
	serviceName := extractString(span, "service_name")
	operation := extractString(span, "operation_name")
	duration := extractString(span, "duration")
	summary := serviceName + " " + operation + " " + duration
	return contract.ToolResult{
		Name:    "slowest_span",
		Summary: summary,
		Score:   score,
		Payload: span,
	}
}

func extractString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractFloat extracts a float64 value from a map.
func extractFloat(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		case json_Number:
			f, _ := val.Float64()
			return f
		}
	}
	return 0
}

type json_Number = interface {
	Float64() (float64, error)
}

