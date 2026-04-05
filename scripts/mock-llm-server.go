// Mock LLM Server for E2E testing
// Starts an OpenAI-compatible API that returns realistic RCA output
// Usage: go run scripts/mock-llm-server.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func main() {
	rcaOutput := map[string]any{
		"summary": "Redis connection pool exhaustion causing API Gateway timeouts — the max-pool of 50 connections is saturated under a 3x traffic spike, leading to cascading 5xx errors across all downstream services.",
		"root_cause": "The Redis connection pool in api-gateway was configured with max_connections=50 in deployment v3.2.1. A traffic spike to 3x normal volume at 09:45 UTC exhausted all available connections, causing new requests to timeout waiting for a connection. The pool exhaustion propagated to payment-service and order-service through synchronous Redis calls, creating a cascading failure.",
		"confidence": 0.87,
		"hypotheses": []map[string]any{
			{"id": "h1", "description": "Redis pool exhaustion under high traffic", "likelihood": 0.87, "evidence_ids": []string{"ev_001", "ev_002", "ev_003"}},
			{"id": "h2", "description": "Network partition between api-gateway and Redis cluster", "likelihood": 0.3, "evidence_ids": []string{"ev_001"}},
		},
		"evidence_ids": []string{"ev_001", "ev_002", "ev_003"},
		"blast_radius": []string{"api-gateway", "payment-service", "order-service", "notification-service"},
		"actions": map[string]any{
			"immediate":  []string{"Increase Redis connection pool to 200 in api-gateway config", "Enable circuit breaker on Redis calls with 50% error threshold"},
			"fix":        []string{"Implement connection pool monitoring with auto-scaling", "Add adaptive pool sizing based on traffic patterns"},
			"prevention": []string{"Add Redis pool utilization alert at 70% threshold", "Implement connection pool health check endpoint", "Load test with 5x traffic before deployment"},
		},
		"timeline": []map[string]any{
			{"time": "2025-01-15T09:30:00Z", "type": "deploy", "service": "api-gateway", "description": "Deployment v3.2.1 rolled out with Redis pool config max_connections=50", "severity": "info"},
			{"time": "2025-01-15T09:45:00Z", "type": "symptom", "service": "api-gateway", "description": "Redis latency P99 starts climbing from 2ms to 8500ms", "severity": "warning"},
			{"time": "2025-01-15T09:55:00Z", "type": "error", "service": "api-gateway", "description": "312 'connection pool exhausted' errors in logs", "severity": "error"},
			{"time": "2025-01-15T10:00:00Z", "type": "alert", "service": "api-gateway", "description": "PagerDuty alert triggered: API-Gateway High Error Rate > 5%", "severity": "critical"},
			{"time": "2025-01-15T10:02:00Z", "type": "action", "service": "platform-team", "description": "Runbook executed: Increase Redis pool and enable circuit breaker", "severity": "info"},
		},
		"uncertainties": []string{"Cannot confirm if traffic spike is organic or caused by a batch job", "Unknown if Redis server itself is experiencing resource pressure"},
	}

	rcaJSON, _ := json.Marshal(rcaOutput)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		resp := map[string]any{
			"id": "chatcmpl-e2e-demo",
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": string(rcaJSON),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     150,
				"completion_tokens": 80,
				"total_tokens":      230,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	addr := ":9999"
	fmt.Printf("Mock LLM server running on %s\n", addr)
	fmt.Printf("Use: LLM_BASE_URL=http://localhost%s LLM_API_KEY=mock\n", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
