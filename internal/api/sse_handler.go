package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// streamAnalysis handles GET /api/v1/incidents/{id}/analyze/stream.
// It streams SSE events during the RCA analysis pipeline.
func (h *handler) streamAnalysis(w http.ResponseWriter, r *http.Request) {
	if h.analysisSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis service not configured", "INTERNAL_ERROR")
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid incident id", "INVALID_REQUEST")
		return
	}

	// Set SSE headers
	setupSSEHeaders(w)

	// Write HTTP 200 status before flushing
	w.WriteHeader(http.StatusOK)

	sse := newSSEWriter(w)

	// Emit initial status event
	if err := sse.WriteEvent("status", map[string]string{
		"message": "analysis started",
		"phase":   "init",
	}); err != nil {
		h.sseLogError("write init status", err)
		return
	}

	// Emit progress: collecting evidence
	if err := sse.WriteEvent("status", map[string]string{
		"message": "collecting evidence",
		"phase":   "evidence",
	}); err != nil {
		h.sseLogError("write evidence status", err)
		return
	}

	// Emit progress: 25%
	if err := sse.WriteEvent("progress", map[string]any{
		"percent": 25,
		"phase":   "evidence",
	}); err != nil {
		h.sseLogError("write progress 25", err)
		return
	}

	// Emit progress: calling LLM
	if err := sse.WriteEvent("status", map[string]string{
		"message": "calling LLM for root cause analysis",
		"phase":   "llm",
	}); err != nil {
		h.sseLogError("write llm status", err)
		return
	}

	// Emit progress: 50%
	if err := sse.WriteEvent("progress", map[string]any{
		"percent": 50,
		"phase":   "llm",
	}); err != nil {
		h.sseLogError("write progress 50", err)
		return
	}

	// Check for client disconnect before the heavy analysis call
	select {
	case <-r.Context().Done():
		h.sseLogError("client disconnected before analysis", r.Context().Err())
		return
	default:
	}

	// Run the actual analysis
	report, err := h.analysisSvc.AnalyzeIncident(r.Context(), id)
	if err != nil {
		slog.Error("analysis failed", "incident_id", id, "error", err)
		if writeErr := sse.WriteError("analysis failed: internal error"); writeErr != nil {
			h.sseLogError("write error event", writeErr)
		}
		return
	}

	// Emit progress: 75%
	if err := sse.WriteEvent("progress", map[string]any{
		"percent": 75,
		"phase":   "report",
	}); err != nil {
		h.sseLogError("write progress 75", err)
		return
	}

	// Emit progress: 90%
	if err := sse.WriteEvent("progress", map[string]any{
		"percent": 90,
		"phase":   "finalizing",
	}); err != nil {
		h.sseLogError("write progress 90", err)
		return
	}

	// Emit 100% progress before complete
	if err := sse.WriteEvent("progress", map[string]any{
		"percent": 100,
		"phase":   "done",
	}); err != nil {
		h.sseLogError("write progress 100", err)
		return
	}

	// Emit the complete event with the full report (must be last)
	if err := sse.WriteEvent("complete", report); err != nil {
		h.sseLogError("write complete event", err)
		return
	}
}

// sseLogError logs an SSE-related error.
func (h *handler) sseLogError(context string, err error) {
	slog.Error("SSE handler error",
		"context", context,
		"error", err,
	)
}
