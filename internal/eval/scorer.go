package eval

// ScoreResult holds multi-dimension evaluation scores for a single RCA output.
type ScoreResult struct {
	CaseID          string  `json:"case_id"`
	PromptVersion   string  `json:"prompt_version"`
	Model           string  `json:"model"`
	Accuracy        float64 `json:"accuracy"`         // root cause match score
	Grounding       float64 `json:"grounding"`         // evidence grounding ratio
	Actionability   float64 `json:"actionability"`     // action quality score
	ConfidenceError float64 `json:"confidence_error"`  // |predicted_conf - actual|
	LatencyMs       int64   `json:"latency_ms"`        // total RCA latency
	TotalScore      float64 `json:"total_score"`       // weighted composite
}

// ScoreWeights defines the contribution of each dimension to TotalScore.
var ScoreWeights = struct {
	Accuracy      float64
	Grounding     float64
	Actionability float64
	Latency       float64
}{
	Accuracy:      0.40,
	Grounding:     0.25,
	Actionability: 0.20,
	Latency:       0.15,
}

// ComputeTotal calculates the weighted composite score.
func ComputeTotal(s ScoreResult) float64 {
	// Normalize latency: 0ms=1.0, 15000ms+=0.0
	latencyScore := 1.0
	if s.LatencyMs > 0 {
		latencyScore = max(0, 1.0-float64(s.LatencyMs)/15000.0)
	}
	return ScoreWeights.Accuracy*s.Accuracy +
		ScoreWeights.Grounding*s.Grounding +
		ScoreWeights.Actionability*s.Actionability +
		ScoreWeights.Latency*latencyScore
}
