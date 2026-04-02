package prompt

import (
	"encoding/json"
	"fmt"
)

// Validator checks RCA prompt output against the expected JSON schema.
type Validator struct{}

// RCAOutput represents the expected JSON output schema for RCA prompts.
type RCAOutput struct {
	Summary       string        `json:"summary"`
	RootCause     string        `json:"root_cause"`
	Confidence    float64       `json:"confidence"`
	Hypotheses    []Hypothesis  `json:"hypotheses"`
	EvidenceIDs   []string      `json:"evidence_ids"`
	BlastRadius   []string      `json:"blast_radius"`
	Actions       Actions       `json:"actions"`
	Uncertainties []string      `json:"uncertainties"`
}

// Hypothesis represents a root cause hypothesis.
type Hypothesis struct {
	Description string   `json:"description"`
	EvidenceIDs []string `json:"evidence_ids"`
	Likelihood  float64  `json:"likelihood"`
}

// Actions represents the three-tier action recommendation structure.
type Actions struct {
	Immediate  []string `json:"immediate"`
	Fix        []string `json:"fix"`
	Prevention []string `json:"prevention"`
}

// Validate checks that a raw JSON string conforms to the RCA output schema.
func (v *Validator) Validate(raw string) (*RCAOutput, error) {
	var out RCAOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("prompt output is not valid JSON: %w", err)
	}
	if err := v.validateFields(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (v *Validator) validateFields(out *RCAOutput) error {
	if out.Summary == "" {
		return fmt.Errorf("summary is required")
	}
	if out.RootCause == "" {
		return fmt.Errorf("root_cause is required")
	}
	if out.Confidence < 0 || out.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1, got %f", out.Confidence)
	}
	if len(out.Actions.Immediate) == 0 {
		return fmt.Errorf("at least one immediate action is required")
	}
	if len(out.Actions.Fix) == 0 {
		return fmt.Errorf("at least one fix action is required")
	}
	if len(out.Actions.Prevention) == 0 {
		return fmt.Errorf("at least one prevention action is required")
	}
	return nil
}
