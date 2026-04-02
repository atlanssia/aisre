package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

func (h *handler) createIncident(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	resp, err := h.svc.CreateIncident(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *handler) getIncident(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id", "INVALID_REQUEST")
		return
	}

	inc, err := h.svc.GetIncident(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
		return
	}

	json.NewEncoder(w).Encode(inc)
}

func (h *handler) listIncidents(w http.ResponseWriter, r *http.Request) {
	filter := store.IncidentFilter{
		Service:  r.URL.Query().Get("service"),
		Severity: r.URL.Query().Get("severity"),
		Status:   r.URL.Query().Get("status"),
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		filter.Limit, _ = strconv.Atoi(limit)
	}
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		filter.Offset, _ = strconv.Atoi(offset)
	}

	items, err := h.svc.ListIncidents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}
	if items == nil {
		items = []contract.Incident{}
	}
	json.NewEncoder(w).Encode(items)
}

func writeError(w http.ResponseWriter, status int, msg, code string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(contract.ErrorResponse{Error: msg, Code: code})
}
