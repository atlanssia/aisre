package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/atlanssia/aisre/internal/contract"
)

// OOClientRebuilder lets the config handler rebuild the O2 client at runtime.
type OOClientRebuilder interface {
	Rebuild(cfg contract.UpdateOOConfig) error
	GetConfig() contract.OOConfig
}

// ConfigService provides config read/write for the API layer.
type ConfigService interface {
	GetAppConfig() contract.AppConfig
	UpdateOOConfig(cfg contract.UpdateOOConfig) error
}

// ooConfigStore holds the current O2 config and can rebuild the client.
type ooConfigStore struct {
	mu       sync.RWMutex
	current  contract.OOConfig
	rebuilder OOClientRebuilder
	llm      contract.LLMConfig
}

// NewConfigService creates a ConfigService with initial values.
func NewConfigService(ooCfg contract.OOConfig, llmCfg contract.LLMConfig, rebuilder OOClientRebuilder) ConfigService {
	return &ooConfigStore{
		current:   ooCfg,
		rebuilder: rebuilder,
		llm:       llmCfg,
	}
}

func (s *ooConfigStore) GetAppConfig() contract.AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return contract.AppConfig{
		OpenObserve: s.current,
		LLM:         s.llm,
	}
}

func (s *ooConfigStore) UpdateOOConfig(cfg contract.UpdateOOConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.rebuilder.Rebuild(cfg); err != nil {
		return err
	}

	s.current = contract.OOConfig{
		BaseURL:  cfg.BaseURL,
		OrgID:    cfg.OrgID,
		Stream:   cfg.Stream,
		Username: cfg.Username,
		Password: cfg.Password,
	}
	return nil
}

func (h *handler) getConfig(w http.ResponseWriter, _ *http.Request) {
	if h.configSvc == nil {
		writeError(w, http.StatusNotFound, "config service not available", contract.ErrCodeFeatureDisabled)
		return
	}
	cfg := h.configSvc.GetAppConfig()
	_ = json.NewEncoder(w).Encode(cfg)
}

func (h *handler) updateOOConfig(w http.ResponseWriter, r *http.Request) {
	if h.configSvc == nil {
		writeError(w, http.StatusNotFound, "config service not available", contract.ErrCodeFeatureDisabled)
		return
	}

	var req contract.UpdateOOConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", contract.ErrCodeInvalidBody)
		return
	}

	if req.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "base_url is required", contract.ErrCodeMissingFields)
		return
	}
	if req.OrgID == "" {
		writeError(w, http.StatusBadRequest, "org_id is required", contract.ErrCodeMissingFields)
		return
	}

	if err := h.configSvc.UpdateOOConfig(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), contract.ErrCodeIngestError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
