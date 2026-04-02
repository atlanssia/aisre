package eval

import "fmt"

// ReleaseGate defines the thresholds that must be met before a prompt can go active.
type ReleaseGate struct {
	MinAccuracy      float64 `json:"min_accuracy"`
	MinGrounding     float64 `json:"min_grounding"`
	MaxHallucination float64 `json:"max_hallucination"`
	MaxP95LatencyMs  int64   `json:"max_p95_latency_ms"`
}

// DefaultReleaseGate returns the production release gate thresholds.
func DefaultReleaseGate() ReleaseGate {
	return ReleaseGate{
		MinAccuracy:      0.85,
		MinGrounding:     0.90,
		MaxHallucination: 0.05,
		MaxP95LatencyMs:  15000,
	}
}

// GateResult holds the outcome of a release gate check.
type GateResult struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures,omitempty"`
}

// Check evaluates a set of scores against the release gate thresholds.
func (g ReleaseGate) Check(scores []ScoreResult) GateResult {
	result := GateResult{Passed: true}

	var totalAccuracy, totalGrounding float64
	var maxLatency int64
	var hallucinationCount int

	for _, s := range scores {
		totalAccuracy += s.Accuracy
		totalGrounding += s.Grounding
		if s.LatencyMs > maxLatency {
			maxLatency = s.LatencyMs
		}
		if s.Grounding < 0.5 {
			hallucinationCount++
		}
	}
	n := len(scores)
	if n == 0 {
		return GateResult{Passed: false, Failures: []string{"no scores to evaluate"}}
	}

	avgAccuracy := totalAccuracy / float64(n)
	avgGrounding := totalGrounding / float64(n)
	hallucinationRate := float64(hallucinationCount) / float64(n)

	if avgAccuracy < g.MinAccuracy {
		result.Passed = false
		result.Failures = append(result.Failures,
			fmt.Sprintf("accuracy %.2f < %.2f", avgAccuracy, g.MinAccuracy))
	}
	if avgGrounding < g.MinGrounding {
		result.Passed = false
		result.Failures = append(result.Failures,
			fmt.Sprintf("grounding %.2f < %.2f", avgGrounding, g.MinGrounding))
	}
	if hallucinationRate > g.MaxHallucination {
		result.Passed = false
		result.Failures = append(result.Failures,
			fmt.Sprintf("hallucination %.2f > %.2f", hallucinationRate, g.MaxHallucination))
	}
	if maxLatency > g.MaxP95LatencyMs {
		result.Passed = false
		result.Failures = append(result.Failures,
			fmt.Sprintf("p95 latency %dms > %dms", maxLatency, g.MaxP95LatencyMs))
	}

	return result
}
