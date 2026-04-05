package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/atlanssia/aisre/internal/contract"
)

// ChangeService defines the interface the API layer depends on.
type ChangeService interface {
	GetChanges(ctx context.Context, q contract.ChangeQuery) ([]contract.ChangeEvent, error)
	GetChangesForIncident(ctx context.Context, incidentID int64) (*contract.ChangeCorrelation, error)
	IngestChange(ctx context.Context, evt contract.ChangeEvent) (int64, error)
}

func (h *handler) listChanges(w http.ResponseWriter, r *http.Request) {
	if h.changeSvc == nil {
		writeError(w, http.StatusNotFound, "change correlation feature not enabled", "FEATURE_DISABLED")
		return
	}

	q := contract.ChangeQuery{
		Service:   r.URL.Query().Get("service"),
		StartTime: r.URL.Query().Get("start_time"),
		EndTime:   r.URL.Query().Get("end_time"),
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			q.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			q.Offset = n
		}
	}

	results, err := h.changeSvc.GetChanges(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "CHANGE_ERROR")
		return
	}
	if results == nil {
		results = []contract.ChangeEvent{}
	}
	_ = json.NewEncoder(w).Encode(results)
}

func (h *handler) getChangesForIncident(w http.ResponseWriter, r *http.Request) {
	if h.changeSvc == nil {
		writeError(w, http.StatusNotFound, "change correlation feature not enabled", "FEATURE_DISABLED")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid incident id", "INVALID_ID")
		return
	}

	corr, err := h.changeSvc.GetChangesForIncident(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "CHANGE_ERROR")
		return
	}
	_ = json.NewEncoder(w).Encode(corr)
}

func (h *handler) ingestChange(w http.ResponseWriter, r *http.Request) {
	if h.changeSvc == nil {
		writeError(w, http.StatusNotFound, "change correlation feature not enabled", "FEATURE_DISABLED")
		return
	}

	var evt contract.ChangeEvent
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	id, err := h.changeSvc.IngestChange(r.Context(), evt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "CHANGE_ERROR")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id})
}
