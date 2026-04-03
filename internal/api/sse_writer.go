package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SSEWriter writes Server-Sent Events to an http.ResponseWriter.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// newSSEWriter creates an SSEWriter that writes events to w.
// It panics if w does not implement http.Flusher.
func newSSEWriter(w http.ResponseWriter) *SSEWriter {
	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("SSEWriter: response writer does not implement http.Flusher")
	}
	return &SSEWriter{w: w, flusher: flusher}
}

// WriteEvent writes a single SSE event with the given type and data payload.
// Data is JSON-encoded. The event is flushed immediately.
func (s *SSEWriter) WriteEvent(eventType string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sse_writer: marshal event data: %w", err)
	}

	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", eventType, jsonData); err != nil {
		return fmt.Errorf("sse_writer: write event: %w", err)
	}

	s.flusher.Flush()
	return nil
}

// WriteError writes an SSE error event with the given message.
func (s *SSEWriter) WriteError(msg string) error {
	return s.WriteEvent("error", map[string]string{"error": msg})
}

// setupSSEHeaders sets the required SSE headers on the response writer.
func setupSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}
