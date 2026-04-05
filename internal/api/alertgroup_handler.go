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

// AlertGroupService defines the interface the API layer depends on.
type AlertGroupService interface {
	Ingest(ctx context.Context, alert contract.IncomingAlert) (*contract.AlertGroup, error)
	List(ctx context.Context, filter contract.AlertGroupFilter) ([]contract.AlertGroup, error)
	Get(ctx context.Context, id int64) (*contract.AlertGroup, error)
	Escalate(ctx context.Context, id int64) (*contract.EscalateResponse, error)
}

func (h *handler) ingestAlert(w http.ResponseWriter, r *http.Request) {
	if h.alertGroupSvc == nil {
		writeError(w, http.StatusNotFound, "alert aggregation feature not enabled", "FEATURE_DISABLED")
		return
	}
	var req contract.IncomingAlert
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	group, err := h.alertGroupSvc.Ingest(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INGEST_ERROR")
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(group)
}

func (h *handler) listAlertGroups(w http.ResponseWriter, r *http.Request) {
	if h.alertGroupSvc == nil {
		writeError(w, http.StatusNotFound, "alert aggregation feature not enabled", "FEATURE_DISABLED")
		return
	}
	filter := contract.AlertGroupFilter{
		Severity:  r.URL.Query().Get("severity"),
		StartTime: r.URL.Query().Get("start_time"),
		EndTime:   r.URL.Query().Get("end_time"),
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = v
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = v
		}
	}

	groups, err := h.alertGroupSvc.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "LIST_ERROR")
		return
	}
	if groups == nil {
		groups = []contract.AlertGroup{}
	}
	_ = json.NewEncoder(w).Encode(groups)}

func (h *handler) getAlertGroup(w http.ResponseWriter, r *http.Request) {
	if h.alertGroupSvc == nil {
		writeError(w, http.StatusNotFound, "alert aggregation feature not enabled", "FEATURE_DISABLED")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert group id", "INVALID_ID")
		return
	}
	group, err := h.alertGroupSvc.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
		return
	}
	_ = json.NewEncoder(w).Encode(group)}

func (h *handler) escalateAlertGroup(w http.ResponseWriter, r *http.Request) {
	if h.alertGroupSvc == nil {
		writeError(w, http.StatusNotFound, "alert aggregation feature not enabled", "FEATURE_DISABLED")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert group id", "INVALID_ID")
		return
	}
	resp, err := h.alertGroupSvc.Escalate(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "already escalated") {
			writeError(w, http.StatusConflict, err.Error(), "ALREADY_ESCALATED")
			return
		}
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "ESCALATE_ERROR")
		return
	}
	_ = json.NewEncoder(w).Encode(resp)}
