package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/atlanssia/aisre/internal/contract"
)

func (h *handler) analyzeIncident(w http.ResponseWriter, r *http.Request) {
	if h.analysisSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis service not configured", "INTERNAL_ERROR")
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid incident id", "INVALID_REQUEST")
		return
	}

	report, err := h.analysisSvc.AnalyzeIncident(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
		return
	}

	_ = json.NewEncoder(w).Encode(report)
}

func (h *handler) getReport(w http.ResponseWriter, r *http.Request) {
	if h.analysisSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis service not configured", "INTERNAL_ERROR")
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report id", "INVALID_REQUEST")
		return
	}

	report, err := h.analysisSvc.GetReport(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
		return
	}

	_ = json.NewEncoder(w).Encode(report)
}

func (h *handler) getEvidence(w http.ResponseWriter, r *http.Request) {
	if h.analysisSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis service not configured", "INTERNAL_ERROR")
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report id", "INVALID_REQUEST")
		return
	}

	items, err := h.analysisSvc.GetEvidence(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}
	if items == nil {
		items = []contract.EvidenceItem{}
	}

	// Return empty array for no evidence (not null)
	if len(items) == 0 {
		_, _ = w.Write([]byte("[]"))
		return
	}

	_ = json.NewEncoder(w).Encode(items)
}
