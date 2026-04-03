#!/bin/bash
# E2E Demo Script — Full alert → RCA pipeline
# Requires: aisre server running, mock LLM or real LLM API key
set -e

BASE_URL="http://localhost:8080"

echo "=== AISRE Phase 1 MVP E2E Demo ==="
echo ""

# Step 1: Create an incident
echo ">>> Step 1: Create Incident via API"
INCIDENT=$(curl -s -X POST "$BASE_URL/api/v1/incidents" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "prometheus",
    "service": "api-gateway",
    "severity": "critical",
    "time_range": "last_15m",
    "trace_id": "trace-demo-001"
  }')

echo "Response: $INCIDENT"
INCIDENT_ID=$(echo "$INCIDENT" | python3 -c "import sys,json; print(json.load(sys.stdin)['incident_id'])" 2>/dev/null || echo "1")
echo "Incident ID: $INCIDENT_ID"
echo ""

# Step 2: Trigger analysis (SSE streaming)
echo ">>> Step 2: Trigger RCA Analysis (SSE)"
echo "Connecting to SSE stream..."
curl -s -N "$BASE_URL/api/v1/incidents/$INCIDENT_ID/analyze/stream" \
  --max-time 120 2>&1 | head -30 || true
echo ""
echo ""

# Step 3: Get the report
echo ">>> Step 3: Fetch RCA Report"
REPORT=$(curl -s "$BASE_URL/api/v1/reports/1")
echo "$REPORT" | python3 -m json.tool 2>/dev/null || echo "$REPORT"
echo ""

# Step 4: Submit feedback
echo ">>> Step 4: Submit Feedback"
FEEDBACK=$(curl -s -X POST "$BASE_URL/api/v1/reports/1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "rating": 4,
    "comment": "Good analysis, root cause identified correctly",
    "user_id": "demo-user",
    "action_taken": "accepted"
  }')
echo "Feedback: $FEEDBACK"
echo ""

# Step 5: Check health
echo ">>> Step 5: Health Check"
curl -s "$BASE_URL/health" && echo ""
echo ""

echo "=== Demo Complete ==="
echo "Open http://localhost:8080 in your browser to view the Workbench UI"
