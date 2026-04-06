package openobserve

import (
	"fmt"

	"github.com/atlanssia/aisre/internal/contract"
)

// mapLogHit maps a raw OO log hit to a ToolResult.
func mapLogHit(hit map[string]any, score float64) contract.ToolResult {
	summary := extractString(hit, "message")
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
	serviceName := extractString(span, "service")
	operation := extractString(span, "span_id")
	duration := extractDuration(span["duration_ms"])
	summary := serviceName + " " + operation + " " + duration + "ms"
	return contract.ToolResult{
		Name:    "slowest_span",
		Summary: summary,
		Score:   score,
		Payload: span,
	}
}

// extractDuration safely formats a duration_ms value which may be nil, float64, or int.
func extractDuration(v any) string {
	if v == nil {
		return "?"
	}
	switch d := v.(type) {
	case float64:
		return fmt.Sprintf("%.0f", d)
	case int:
		return fmt.Sprintf("%d", d)
	case int64:
		return fmt.Sprintf("%d", d)
	case string:
		return d
	default:
		return fmt.Sprintf("%v", d)
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


