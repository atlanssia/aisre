package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/atlanssia/aisre/internal/contract"
)

// PromptStudioService defines the interface the API layer depends on.
type PromptStudioService interface {
	List(ctx context.Context) ([]contract.PromptTemplate, error)
	Get(ctx context.Context, id int64) (*contract.PromptTemplate, error)
	Create(ctx context.Context, req contract.CreatePromptTemplateRequest) (*contract.PromptTemplate, error)
	Update(ctx context.Context, id int64, req contract.UpdatePromptTemplateRequest) (*contract.PromptTemplate, error)
	DryRun(ctx context.Context, id int64, vars map[string]string) (string, error)
}

func (h *handler) listPromptTemplates(w http.ResponseWriter, r *http.Request) {
	if h.promptStudioSvc == nil {
		writeError(w, http.StatusNotFound, "prompt studio feature not enabled", "FEATURE_DISABLED")
		return
	}
	templates, err := h.promptStudioSvc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "PROMPT_STUDIO_ERROR")
		return
	}
	if templates == nil {
		templates = []contract.PromptTemplate{}
	}
	json.NewEncoder(w).Encode(templates)
}

func (h *handler) getPromptTemplate(w http.ResponseWriter, r *http.Request) {
	if h.promptStudioSvc == nil {
		writeError(w, http.StatusNotFound, "prompt studio feature not enabled", "FEATURE_DISABLED")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid template id", "INVALID_ID")
		return
	}
	tpl, err := h.promptStudioSvc.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
		return
	}
	json.NewEncoder(w).Encode(tpl)
}

func (h *handler) createPromptTemplate(w http.ResponseWriter, r *http.Request) {
	if h.promptStudioSvc == nil {
		writeError(w, http.StatusNotFound, "prompt studio feature not enabled", "FEATURE_DISABLED")
		return
	}
	var req contract.CreatePromptTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.Name == "" || req.Stage == "" {
		writeError(w, http.StatusBadRequest, "name and stage are required", "MISSING_FIELDS")
		return
	}
	tpl, err := h.promptStudioSvc.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "VALIDATION_ERROR")
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tpl)
}

func (h *handler) updatePromptTemplate(w http.ResponseWriter, r *http.Request) {
	if h.promptStudioSvc == nil {
		writeError(w, http.StatusNotFound, "prompt studio feature not enabled", "FEATURE_DISABLED")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid template id", "INVALID_ID")
		return
	}
	var req contract.UpdatePromptTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	tpl, err := h.promptStudioSvc.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "VALIDATION_ERROR")
		return
	}
	json.NewEncoder(w).Encode(tpl)
}

func (h *handler) dryRunPromptTemplate(w http.ResponseWriter, r *http.Request) {
	if h.promptStudioSvc == nil {
		writeError(w, http.StatusNotFound, "prompt studio feature not enabled", "FEATURE_DISABLED")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid template id", "INVALID_ID")
		return
	}
	var req contract.PromptDryRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	result, err := h.promptStudioSvc.DryRun(r.Context(), id, req.Variables)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "DRYRUN_ERROR")
		return
	}
	json.NewEncoder(w).Encode(contract.PromptDryRunResponse{
		UserPrompt: result,
	})
}
