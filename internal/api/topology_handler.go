package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/atlanssia/aisre/internal/contract"
)

// TopologyService defines the interface the API layer depends on.
type TopologyService interface {
	GetTopology(ctx context.Context) (*contract.TopologyGraph, error)
	ComputeBlastRadius(ctx context.Context, service string, depth int) ([]contract.BlastRadiusAffected, error)
	ComputeBlastRadiusForIncident(ctx context.Context, incidentID int64, depth int) (*contract.BlastRadiusResponse, error)
	AddEdge(ctx context.Context, req contract.AddEdgeRequest) (int64, error)
}

func (h *handler) getTopology(w http.ResponseWriter, r *http.Request) {
	if h.topoSvc == nil {
		writeError(w, http.StatusNotFound, "topology feature not enabled", "FEATURE_DISABLED")
		return
	}

	graph, err := h.topoSvc.GetTopology(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "TOPOLOGY_ERROR")
		return
	}
	json.NewEncoder(w).Encode(graph)
}

func (h *handler) getBlastRadius(w http.ResponseWriter, r *http.Request) {
	if h.topoSvc == nil {
		writeError(w, http.StatusNotFound, "topology feature not enabled", "FEATURE_DISABLED")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid incident id", "INVALID_ID")
		return
	}

	depth := 3
	if v := r.URL.Query().Get("depth"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			depth = n
		}
	}

	resp, err := h.topoSvc.ComputeBlastRadiusForIncident(r.Context(), id, depth)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "BLAST_RADIUS_ERROR")
		return
	}
	json.NewEncoder(w).Encode(resp)
}

func (h *handler) addTopologyEdge(w http.ResponseWriter, r *http.Request) {
	if h.topoSvc == nil {
		writeError(w, http.StatusNotFound, "topology feature not enabled", "FEATURE_DISABLED")
		return
	}

	var req contract.AddEdgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	if req.Source == "" || req.Target == "" {
		writeError(w, http.StatusBadRequest, "source and target are required", "MISSING_FIELDS")
		return
	}

	id, err := h.topoSvc.AddEdge(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "TOPOLOGY_ERROR")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": id})
}
