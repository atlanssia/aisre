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

// EmbeddingConfig holds configuration for the embedding API client.
// Independent from LLMConfig — different provider, endpoint, and credentials.
type EmbeddingConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	Dimensions int
}

// embeddingRequest is the request body for the embeddings API.
type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embeddingResponse is the response from the embeddings API.
type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// EmbeddingClient calls OpenAI-compatible embedding APIs.
type EmbeddingClient struct {
	cfg  EmbeddingConfig
	http *http.Client
}

// NewEmbeddingClient creates a new embedding client with the given configuration.
func NewEmbeddingClient(cfg EmbeddingConfig) *EmbeddingClient {
	return &EmbeddingClient{
		cfg: cfg,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Model returns the configured model name.
func (c *EmbeddingClient) Model() string {
	return c.cfg.Model
}

// Embed calls the embedding API and returns vectors for the given texts.
func (c *EmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := embeddingRequest{
		Model: c.cfg.Model,
		Input: texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("embedding_client: marshal request: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding_client: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding_client: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("embedding_client: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding_client: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("embedding_client: unmarshal response: %w", err)
	}

	result := make([][]float64, len(embResp.Data))
	for _, d := range embResp.Data {
		result[d.Index] = d.Embedding
	}
	return result, nil
}
