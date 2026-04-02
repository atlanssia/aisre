package contract

// ToolResult is the unified output of a tool execution.
type ToolResult struct {
	Name    string         `json:"name"`
	Summary string         `json:"summary"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}
