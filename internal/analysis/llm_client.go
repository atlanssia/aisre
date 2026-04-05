package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMConfig holds configuration for the LLM client.
type LLMConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	MaxTokens int
}

// Message represents a chat message sent to the LLM.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse is the parsed response from the LLM.
type LLMResponse struct {
	Content string
	Usage   TokenUsage
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Hypothesis represents a root cause hypothesis.
type Hypothesis struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Likelihood  float64  `json:"likelihood"`
	EvidenceIDs []string `json:"evidence_ids"`
}

// Actions represents recommended actions grouped by urgency.
type Actions struct {
	Immediate  []string `json:"immediate"`
	Fix        []string `json:"fix"`
	Prevention []string `json:"prevention"`
}

// TimelineEntry is a structured timeline event from the LLM.
type TimelineEntry struct {
	Time        string  `json:"time"`
	Type        string  `json:"type"` // "symptom", "error", "deploy", "alert", "action"
	Service     string  `json:"service"`
	Description string  `json:"description"`
	Severity    string  `json:"severity,omitempty"`
}

// RCAOutput is the structured output expected from the LLM.
type RCAOutput struct {
	Summary       string          `json:"summary"`
	RootCause     string          `json:"root_cause"`
	Confidence    float64         `json:"confidence"`
	Hypotheses    []Hypothesis    `json:"hypotheses"`
	EvidenceIDs   []string        `json:"evidence_ids"`
	BlastRadius   []string        `json:"blast_radius"`
	Actions       Actions         `json:"actions"`
	Timeline      []TimelineEntry `json:"timeline"`
	Uncertainties []string        `json:"uncertainties"`
}

// openaiRequest is the request body sent to OpenAI-compatible APIs.
type openaiRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

// openaiResponse is the raw response from OpenAI-compatible APIs.
type openaiResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage TokenUsage `json:"usage"`
}

// LLMClient is an OpenAI-compatible API client.
type LLMClient struct {
	cfg    LLMConfig
	http   *http.Client
}

// NewLLMClient creates a new LLM client with the given configuration.
func NewLLMClient(cfg LLMConfig) *LLMClient {
	return &LLMClient{
		cfg: cfg,
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Complete sends messages to the LLM and returns the response.
func (c *LLMClient) Complete(ctx context.Context, messages []Message) (*LLMResponse, error) {
	reqBody := openaiRequest{
		Model:     c.cfg.Model,
		Messages:  messages,
		MaxTokens: c.cfg.MaxTokens,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("llm_client: marshal request: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm_client: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm_client: send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("llm_client: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm_client: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("llm_client: unmarshal response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("llm_client: no choices in response")
	}

	return &LLMResponse{
		Content: openaiResp.Choices[0].Message.Content,
		Usage:   openaiResp.Usage,
	}, nil
}

// ParseRCAOutput parses the LLM response content into a structured RCAOutput.
func (c *LLMClient) ParseRCAOutput(content string) (*RCAOutput, error) {
	var output RCAOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return nil, fmt.Errorf("llm_client: parse RCA output: %w", err)
	}
	return &output, nil
}
