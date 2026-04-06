package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
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
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error(), contract.ErrCodeNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), contract.ErrCodeInternal)
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
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error(), contract.ErrCodeNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), contract.ErrCodeInternal)
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

func (h *handler) listReports(w http.ResponseWriter, r *http.Request) {
	if h.reportRepo == nil {
		writeError(w, http.StatusNotFound, "report listing not available", contract.ErrCodeFeatureDisabled)
		return
	}

	filter := store.ReportFilter{
		Service:  r.URL.Query().Get("service"),
		Severity: r.URL.Query().Get("severity"),
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		v, err := strconv.Atoi(limit)
		if err != nil || v < 0 {
			writeError(w, http.StatusBadRequest, "invalid limit parameter", contract.ErrCodeInvalidRequest)
			return
		}
		filter.Limit = v
	}
	if filter.Limit == 0 {
		filter.Limit = 50
	}

	if offset := r.URL.Query().Get("offset"); offset != "" {
		v, err := strconv.Atoi(offset)
		if err != nil || v < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset parameter", contract.ErrCodeInvalidRequest)
			return
		}
		filter.Offset = v
	}

	reports, err := h.reportRepo.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), contract.ErrCodeInternal)
		return
	}
	if reports == nil {
		reports = []store.Report{}
	}

	_ = json.NewEncoder(w).Encode(reports)
}
