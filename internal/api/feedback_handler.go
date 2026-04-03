package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

func (h *handler) submitFeedback(w http.ResponseWriter, r *http.Request) {
	if h.feedbackRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "feedback service not configured", "INTERNAL_ERROR")
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report id", "INVALID_REQUEST")
		return
	}

	var req contract.FeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if req.Rating < 1 || req.Rating > 5 {
		writeError(w, http.StatusBadRequest, "rating must be between 1 and 5", "INVALID_REQUEST")
		return
	}

	// Verify report exists via analysisSvc
	if h.analysisSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis service not configured", "INTERNAL_ERROR")
		return
	}
	_, err = h.analysisSvc.GetReport(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("report %d not found", id), "NOT_FOUND")
		return
	}

	fb := &store.Feedback{
		ReportID:    id,
		UserID:      req.UserID,
		Rating:      req.Rating,
		Comment:     req.Comment,
		ActionTaken: req.ActionTaken,
	}

	fbID, err := h.feedbackRepo.Create(r.Context(), fb)
	if err != nil {
		slog.Error("submitFeedback failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
		return
	}

	resp := contract.FeedbackResponse{
		ID:          fbID,
		ReportID:    id,
		Rating:      req.Rating,
		Comment:     req.Comment,
		UserID:      req.UserID,
		ActionTaken: req.ActionTaken,
		CreatedAt:   fb.CreatedAt,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *handler) searchReports(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing search query parameter 'q'", "INVALID_REQUEST")
		return
	}

	filter := store.ReportFilter{
		Service:  r.URL.Query().Get("service"),
		Severity: r.URL.Query().Get("severity"),
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		v, err := strconv.Atoi(limit)
		if err != nil || v < 0 {
			writeError(w, http.StatusBadRequest, "invalid limit parameter", "INVALID_REQUEST")
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
			writeError(w, http.StatusBadRequest, "invalid offset parameter", "INVALID_REQUEST")
			return
		}
		filter.Offset = v
	}

	if h.analysisSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis service not configured", "INTERNAL_ERROR")
		return
	}

	// Use the underlying reportRepo from the analysis service if available.
	// For the mock-based approach, we use a dedicated reportRepo on the handler.
	if h.reportRepo != nil {
		reports, err := h.reportRepo.Search(r.Context(), query, filter)
		if err != nil {
			slog.Error("searchReports failed", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
			return
		}

		results := make([]contract.ReportResponse, 0, len(reports))
		for _, rp := range reports {
			results = append(results, contract.ReportResponse{
				ID:         rp.ID,
				IncidentID: rp.IncidentID,
				Summary:    rp.Summary,
				RootCause:  rp.RootCause,
				Confidence: rp.Confidence,
				CreatedAt:  rp.CreatedAt,
			})
		}
		if results == nil {
			results = []contract.ReportResponse{}
		}
		json.NewEncoder(w).Encode(results)
		return
	}

	// Fallback: return empty array if no reportRepo available
	w.Write([]byte("[]"))
}
