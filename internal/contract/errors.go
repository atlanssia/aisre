package contract

// ErrorResponse is the standard error DTO.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Error codes
const (
	ErrCodeInvalidRequest  = "INVALID_REQUEST"
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeInternal        = "INTERNAL_ERROR"
	ErrCodeAdapterTimeout  = "ADAPTER_TIMEOUT"
	ErrCodeLLMFailed       = "LLM_FAILED"
	ErrCodeDuplicate       = "DUPLICATE"
	ErrCodeInvalidBody     = "INVALID_BODY"
	ErrCodeInvalidID       = "INVALID_ID"
	ErrCodeMissingFields   = "MISSING_FIELDS"
	ErrCodeFeatureDisabled = "FEATURE_DISABLED"
	ErrCodeAlreadyExists   = "ALREADY_EXISTS"
	ErrCodeConflict        = "CONFLICT"
	ErrCodeIngestError     = "INGEST_ERROR"
	ErrCodeListError       = "LIST_ERROR"
	ErrCodeEscalateError   = "ESCALATE_ERROR"
	ErrCodeGenerateError   = "GENERATE_ERROR"
	ErrCodeUpdateError     = "UPDATE_ERROR"
	ErrCodeInvalidStatus   = "INVALID_STATUS"
	ErrCodeValidationError = "VALIDATION_ERROR"
	ErrCodeDryRunError     = "DRYRUN_ERROR"
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
