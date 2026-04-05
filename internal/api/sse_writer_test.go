package api

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSSEWriter_WriteEvent(t *testing.T) {
	w := httptest.NewRecorder()
	sse := newSSEWriter(w)

	err := sse.WriteEvent("status", map[string]string{"message": "collecting evidence"})
	if err != nil {
		t.Fatalf("WriteEvent returned error: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: status\n") {
		t.Errorf("expected 'event: status' in body, got:\n%s", body)
	}
	if !strings.Contains(body, `data: {"message":"collecting evidence"}`) {
		t.Errorf("expected data JSON in body, got:\n%s", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Errorf("expected body to end with double newline, got:\n%q", body)
	}
}

func TestSSEWriter_WriteEvent_MultipleEvents(t *testing.T) {
	w := httptest.NewRecorder()
	sse := newSSEWriter(w)

	_ = sse.WriteEvent("status", map[string]string{"phase": "start"})
	_ = sse.WriteEvent("progress", map[string]int{"percent": 50})
	_ = sse.WriteEvent("complete", map[string]string{"id": "123"})

	body := w.Body.String()
	events := strings.Split(strings.TrimSpace(body), "\n\n")
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d:\n%s", len(events), body)
	}

	for i, expected := range []string{"event: status", "event: progress", "event: complete"} {
		if !strings.Contains(events[i], expected) {
			t.Errorf("event %d: expected %q, got:\n%s", i, expected, events[i])
		}
	}
}

func TestSSEWriter_WriteError(t *testing.T) {
	w := httptest.NewRecorder()
	sse := newSSEWriter(w)

	err := sse.WriteError("something went wrong")
	if err != nil {
		t.Fatalf("WriteError returned error: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: error\n") {
		t.Errorf("expected 'event: error' in body, got:\n%s", body)
	}
	if !strings.Contains(body, `"error":"something went wrong"`) {
		t.Errorf("expected error message in data, got:\n%s", body)
	}
}

func TestSSEWriter_SSEHeaders(t *testing.T) {
	// Test that setupSSEHeaders sets the correct headers
	w := httptest.NewRecorder()
	setupSSEHeaders(w)

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control 'no-cache', got %q", cc)
	}
	if conn := w.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("expected Connection 'keep-alive', got %q", conn)
	}
}

func TestSSEWriter_FlushesAfterWrite(t *testing.T) {
	// Use a custom ResponseWriter that tracks flush calls
	w := &flushTracker{ResponseWriter: httptest.NewRecorder()}
	sse := newSSEWriter(w)

	_ = sse.WriteEvent("status", map[string]string{"msg": "test"})

	if w.flushCount == 0 {
		t.Error("expected Flush to be called after WriteEvent")
	}
}

// flushTracker wraps http.ResponseWriter to track Flush calls.
type flushTracker struct {
	http.ResponseWriter
	flushCount int
}

func (f *flushTracker) Flush() {
	f.flushCount++
}

// Write is needed so the embedded data goes to the right place.
func (f *flushTracker) Write(b []byte) (int, error) {
	return f.ResponseWriter.Write(b)
}

// Verify SSEWriter fails gracefully when data cannot be marshalled
func TestSSEWriter_WriteEvent_UnmarshallableData(t *testing.T) {
	w := httptest.NewRecorder()
	sse := newSSEWriter(w)

	err := sse.WriteEvent("status", make(chan int))
	if err == nil {
		t.Error("expected error for unmarshallable data (chan), got nil")
	}
}

// parseSSEEvents is a test helper to parse raw SSE text into event/data pairs.
func parseSSEEvents(raw string) []sseEvent {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	var events []sseEvent
	var current sseEvent
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			current.eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			current.data = strings.TrimPrefix(line, "data: ")
		} else if line == "" && current.eventType != "" {
			events = append(events, current)
			current = sseEvent{}
		}
	}
	return events
}

type sseEvent struct {
	eventType string
	data      string
}
