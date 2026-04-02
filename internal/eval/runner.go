package eval

// Runner executes benchmark suites against RCA analysis results.
type Runner struct {
	gate ReleaseGate
}

// NewRunner creates a benchmark runner with the given release gate.
func NewRunner(gate ReleaseGate) *Runner {
	return &Runner{gate: gate}
}

// RunBenchmark executes all cases in a suite and returns aggregated results.
func (r *Runner) RunBenchmark(suite BenchmarkSuite) ([]ScoreResult, GateResult, error) {
	// TODO: iterate suite.Cases, run RCA, score each, aggregate
	return nil, GateResult{}, nil
}

// CompareVersions runs the same suite with two different prompt/model combos
// and returns a diff report.
func (r *Runner) CompareVersions(suite BenchmarkSuite, v1, v2 Config) (*DiffReport, error) {
	// TODO: run both versions, compare scores
	return nil, nil
}

// Config defines the RCA configuration for a benchmark run.
type Config struct {
	PromptVersion string `json:"prompt_version"`
	Model         string `json:"model"`
}

// DiffReport compares two benchmark runs.
type DiffReport struct {
	V1       Config        `json:"v1"`
	V2       Config        `json:"v2"`
	Scores   []ScoreDiff   `json:"scores"`
	Summary  string        `json:"summary"`
}

// ScoreDiff represents the score difference for a single case.
type ScoreDiff struct {
	CaseID     string  `json:"case_id"`
	V1Score    float64 `json:"v1_score"`
	V2Score    float64 `json:"v2_score"`
	Delta      float64 `json:"delta"`
	Regressed  bool    `json:"regressed"`
}
