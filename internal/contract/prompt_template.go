package contract

// PromptTemplate represents a managed prompt template.
type PromptTemplate struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Stage     string   `json:"stage"` // "context", "evidence", "rca", "summary"
	SystemTpl string   `json:"system_tpl"`
	UserTpl   string   `json:"user_tpl"`
	Variables []string `json:"variables"`
	IsDefault bool     `json:"is_default"`
	Version   int      `json:"version"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// CreatePromptTemplateRequest is the API request to create a prompt template.
type CreatePromptTemplateRequest struct {
	Name      string   `json:"name"`
	Stage     string   `json:"stage"`
	SystemTpl string   `json:"system_tpl"`
	UserTpl   string   `json:"user_tpl"`
	Variables []string `json:"variables"`
	IsDefault bool     `json:"is_default"`
}

// UpdatePromptTemplateRequest is the API request to update a prompt template.
type UpdatePromptTemplateRequest struct {
	Name      string   `json:"name"`
	Stage     string   `json:"stage"`
	SystemTpl string   `json:"system_tpl"`
	UserTpl   string   `json:"user_tpl"`
	Variables []string `json:"variables"`
	IsDefault bool     `json:"is_default"`
}

// PromptDryRunRequest is the API request to dry-run a prompt template.
type PromptDryRunRequest struct {
	Variables map[string]string `json:"variables"`
}

// PromptDryRunResponse is the API response for a prompt template dry-run.
type PromptDryRunResponse struct {
	SystemPrompt string `json:"system_prompt"`
	UserPrompt   string `json:"user_prompt"`
}
