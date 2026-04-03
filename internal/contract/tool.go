package contract

// ToolResult is the unified output of a tool execution.
type ToolResult struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`      // "trace", "log", "metric", "change"
	Summary   string         `json:"summary"`
	Score     float64        `json:"score"`
	SourceURL string         `json:"source_url,omitempty"`
	Payload   map[string]any `json:"payload"`
}
