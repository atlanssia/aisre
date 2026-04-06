package contract

// OOConfig holds OpenObserve connection settings.
type OOConfig struct {
	BaseURL  string `json:"base_url"`
	OrgID    string `json:"org_id"`
	Stream   string `json:"stream"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

// LLMConfig holds LLM provider settings (read-only for now).
type LLMConfig struct {
	Provider     string `json:"provider"`
	BaseURL      string `json:"base_url"`
	RCAModel     string `json:"rca_model"`
	SummaryModel string `json:"summary_model"`
	EmbedModel   string `json:"embed_model"`
}

// AppConfig is the full application configuration exposed via API.
type AppConfig struct {
	OpenObserve OOConfig `json:"openobserve"`
	LLM         LLMConfig `json:"llm"`
}

// UpdateOOConfig is the request body for updating O2 settings.
type UpdateOOConfig struct {
	BaseURL  string `json:"base_url"`
	OrgID    string `json:"org_id"`
	Stream   string `json:"stream"`
	Username string `json:"username"`
	Password string `json:"password"`
}
