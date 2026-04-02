package contract

// ErrorResponse is the standard error DTO.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Error codes
const (
	ErrCodeInvalidRequest = "INVALID_REQUEST"
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeInternal       = "INTERNAL_ERROR"
	ErrCodeAdapterTimeout = "ADAPTER_TIMEOUT"
	ErrCodeLLMFailed      = "LLM_FAILED"
	ErrCodeDuplicate      = "DUPLICATE"
)

// Valid severity levels
var ValidSeverities = map[string]bool{
	"critical": true,
	"high":     true,
	"medium":   true,
	"low":      true,
	"info":     true,
}

// Valid feedback actions
var ValidActions = map[string]bool{
	"accepted": true,
	"partial":  true,
	"rejected": true,
}
