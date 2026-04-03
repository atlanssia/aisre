package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/atlanssia/aisre/internal/contract"
)

func (h *handler) getSimilar(w http.ResponseWriter, r *http.Request) {
	if h.similarSvc == nil {
		writeError(w, http.StatusNotFound, "similar incident feature not enabled", "NOT_FOUND")
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id", "INVALID_REQUEST")
		return
	}

	topK := 5
	if v := r.URL.Query().Get("top_k"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid top_k parameter", "INVALID_REQUEST")
			return
		}
		topK = parsed
	}

	threshold := 0.5
	if v := r.URL.Query().Get("threshold"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid threshold parameter", "INVALID_REQUEST")
			return
		}
		threshold = parsed
	}

	results, err := h.similarSvc.FindSimilar(r.Context(), id, topK, threshold)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("find similar: %s", err), "INTERNAL_ERROR")
		return
	}
	if results == nil {
		results = []contract.SimilarResult{}
	}

	json.NewEncoder(w).Encode(results)
}

func (h *handler) computeEmbedding(w http.ResponseWriter, r *http.Request) {
	if h.similarSvc == nil {
		writeError(w, http.StatusNotFound, "similar incident feature not enabled", "NOT_FOUND")
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id", "INVALID_REQUEST")
		return
	}

	if err := h.similarSvc.ComputeEmbedding(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compute embedding: %s", err), "INTERNAL_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
