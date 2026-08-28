// Package embed provides a minimal client for an OpenAI-compatible
// /v1/embeddings endpoint (LM Studio, Ollama, or any local server that
// implements the same shape). Embeddings never leave the machine running
// this client — there is no cloud provider in the loop.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// batchSize caps how many texts go in a single request. LM Studio and
// Ollama both handle this comfortably for short issue titles and Slack
// messages.
const batchSize = 32

// maxRetries is how many attempts a single batch gets before giving up.
// Local embedding services are reliable; this covers model-load stalls
// and the odd timeout, not systemic outages.
const maxRetries = 4

// Client calls an OpenAI-compatible /v1/embeddings endpoint.
type Client struct {
	baseURL string // e.g. http://localhost:1234/v1 (no trailing slash)
	model   string // e.g. text-embedding-nomic-embed-text-v1.5
	http    *http.Client
}

// Config configures a Client.
type Config struct {
	BaseURL string
	Model   string
}

// New creates a Client for the given OpenAI-compatible endpoint.
func New(cfg Config) *Client {
	return &Client{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Model returns the configured model name. Callers stamp this onto every
// embedded node so a later model swap (different dimension, different
// embedding space) can be detected with a WHERE clause instead of
// silently corrupting similarity scores.
func (c *Client) Model() string {
	return c.model
}

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// Embed returns one embedding vector per input text, in the same order
// as the input. Texts are sent in batches of batchSize; each batch
// retries with exponential backoff on transient failures.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, 0, len(texts))
	for start := 0; start < len(texts); start += batchSize {
		end := min(start+batchSize, len(texts))
		vectors, err := c.embedBatchWithRetry(ctx, texts[start:end])
		if err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", start, end, err)
		}
		out = append(out, vectors...)
	}
	return out, nil
}

func (c *Client) embedBatchWithRetry(ctx context.Context, texts []string) ([][]float64, error) {
	body, err := json.Marshal(embeddingsRequest{Model: c.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	backoff := 500 * time.Millisecond
	for attempt := 1; attempt <= maxRetries; attempt++ {
		vectors, err := c.embedBatch(ctx, body, len(texts))
		if err == nil {
			return vectors, nil
		}
		lastErr = err

		if attempt == maxRetries {
			break
		}
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		backoff *= 2
	}
	return nil, fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

func (c *Client) embedBatch(ctx context.Context, body []byte, want int) ([][]float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed embeddingsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(parsed.Data) != want {
		return nil, fmt.Errorf("expected %d embeddings, got %d", want, len(parsed.Data))
	}

	vectors := make([][]float64, want)
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= want {
			return nil, fmt.Errorf("embedding index %d out of range [0,%d)", d.Index, want)
		}
		vectors[d.Index] = d.Embedding
	}
	for i, v := range vectors {
		if v == nil {
			return nil, fmt.Errorf("missing embedding at index %d", i)
		}
	}
	return vectors, nil
}
