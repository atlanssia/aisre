package eval_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/atlanssia/aisre/internal/eval"
)

func TestGoldenDatasetExists(t *testing.T) {
	suite := loadGoldenSuite(t)

	if suite.Name == "" {
		t.Error("golden suite name is empty")
	}
	if suite.Version == "" {
		t.Error("golden suite version is empty")
	}
	if len(suite.Cases) < 3 {
		t.Errorf("expected at least 3 golden cases, got %d", len(suite.Cases))
	}
}

func TestGoldenCaseFilesExist(t *testing.T) {
	suite := loadGoldenSuite(t)
	goldenDir := filepath.Join("..", "..", "test", "eval", "golden")

	for _, c := range suite.Cases {
		casePath := filepath.Join(goldenDir, c.CaseFile)
		if _, err := os.Stat(casePath); os.IsNotExist(err) {
			t.Errorf("case file %q does not exist at %s", c.CaseFile, casePath)
		}
	}
}

func TestGoldenCaseCategories(t *testing.T) {
	suite := loadGoldenSuite(t)

	expectedCategories := map[string]bool{
		"redis_timeout":  true,
		"db_connection_leak": true,
		"high_error_rate": true,
	}

	for _, c := range suite.Cases {
		if !expectedCategories[c.Category] {
			t.Errorf("unexpected category %q in case %q", c.Category, c.CaseFile)
		}
	}
}

func TestGoldenCasesHaveExpectedRCA(t *testing.T) {
	suite := loadGoldenSuite(t)

	for _, c := range suite.Cases {
		if c.ExpectedRCA == "" {
			t.Errorf("case %q has empty expected_rca", c.CaseFile)
		}
		if c.Category == "" {
			t.Errorf("case %q has empty category", c.CaseFile)
		}
	}
}

func TestScoreComputation(t *testing.T) {
	tests := []struct {
		name     string
		score    eval.ScoreResult
		expected float64
	}{
		{
			name: "perfect score",
			score: eval.ScoreResult{
				Accuracy:      1.0,
				Grounding:     1.0,
				Actionability: 1.0,
				LatencyMs:     0,
			},
			expected: 1.0,
		},
		{
			name: "zero score",
			score: eval.ScoreResult{
				Accuracy:      0.0,
				Grounding:     0.0,
				Actionability: 0.0,
				LatencyMs:     15000,
			},
			expected: 0.0,
		},
		{
			name: "mixed score",
			score: eval.ScoreResult{
				Accuracy:      0.8,
				Grounding:     0.6,
				Actionability: 0.7,
				LatencyMs:     3000,
			},
			expected: 0.40*0.8 + 0.25*0.6 + 0.20*0.7 + 0.15*(1.0-3000.0/15000.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eval.ComputeTotal(tt.score)
			if diff := got - tt.expected; diff > 0.001 || diff < -0.001 {
				t.Errorf("ComputeTotal() = %.4f, want %.4f", got, tt.expected)
			}
		})
	}
}

func loadGoldenSuite(t *testing.T) eval.BenchmarkSuite {
	t.Helper()

	goldenDir := filepath.Join("..", "..", "test", "eval", "golden")
	suitePath := filepath.Join(goldenDir, "suite.json")

	data, err := os.ReadFile(suitePath)
	if err != nil {
		t.Fatalf("read golden suite: %v", err)
	}

	var suite eval.BenchmarkSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("parse golden suite: %v", err)
	}

	return suite
}
