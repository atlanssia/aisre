package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/atlanssia/aisre/internal/contract"
)

// PostmortemService defines the interface the API layer depends on.
type PostmortemService interface {
	Generate(ctx context.Context, incidentID int64) (*contract.Postmortem, error)
	List(ctx context.Context) ([]contract.Postmortem, error)
	Get(ctx context.Context, id int64) (*contract.Postmortem, error)
	Update(ctx context.Context, id int64, req contract.UpdatePostmortemRequest) (*contract.Postmortem, error)
}

func (h *handler) generatePostmortem(w http.ResponseWriter, r *http.Request) {
	if h.postmortemSvc == nil {
		writeError(w, http.StatusNotFound, "postmortem feature not enabled", "FEATURE_DISABLED")
		return
	}
	incidentID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid incident id", "INVALID_ID")
		return
	}

	pm, err := h.postmortemSvc.Generate(r.Context(), incidentID)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, err.Error(), "ALREADY_EXISTS")
			return
		}
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "GENERATE_ERROR")
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pm)
}

func (h *handler) listPostmortems(w http.ResponseWriter, r *http.Request) {
	if h.postmortemSvc == nil {
		writeError(w, http.StatusNotFound, "postmortem feature not enabled", "FEATURE_DISABLED")
		return
	}

	pms, err := h.postmortemSvc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "LIST_ERROR")
		return
	}
	if pms == nil {
		pms = []contract.Postmortem{}
	}
	json.NewEncoder(w).Encode(pms)
}

func (h *handler) getPostmortem(w http.ResponseWriter, r *http.Request) {
	if h.postmortemSvc == nil {
		writeError(w, http.StatusNotFound, "postmortem feature not enabled", "FEATURE_DISABLED")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid postmortem id", "INVALID_ID")
		return
	}

	pm, err := h.postmortemSvc.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
		return
	}
	json.NewEncoder(w).Encode(pm)
}

func (h *handler) updatePostmortem(w http.ResponseWriter, r *http.Request) {
	if h.postmortemSvc == nil {
		writeError(w, http.StatusNotFound, "postmortem feature not enabled", "FEATURE_DISABLED")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid postmortem id", "INVALID_ID")
		return
	}

	var req contract.UpdatePostmortemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	pm, err := h.postmortemSvc.Update(r.Context(), id, req)
	if err != nil {
		if strings.Contains(err.Error(), "invalid transition") || strings.Contains(err.Error(), "invalid status") {
			writeError(w, http.StatusBadRequest, err.Error(), "INVALID_STATUS")
			return
		}
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "UPDATE_ERROR")
		return
	}
	json.NewEncoder(w).Encode(pm)
}
