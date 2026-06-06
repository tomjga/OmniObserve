// Package llm is a vendor-agnostic chat-completion client. It speaks the
// OpenAI-compatible /chat/completions schema, which Gemini, OpenAI, Claude (via a proxy),
// and local runtimes (Ollama, vLLM) all implement. The provider is chosen entirely by
// config — base URL, model, API key — so no vendor name appears in code. See the
// project's [[vendor-agnostic-llm]] decision.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client targets one OpenAI-compatible endpoint. Construct it with New.
type Client struct {
	baseURL string // e.g. https://generativelanguage.googleapis.com/v1beta/openai
	model   string // e.g. gemini-2.0-flash, gpt-4o, llama3.1
	apiKey  string
	http    *http.Client
}

// New builds a client. An empty baseURL or apiKey yields a client whose Configured()
// is false, so callers can degrade gracefully (skip RCA) instead of erroring.
func New(baseURL, model, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Configured reports whether enough is set to make a call.
func (c *Client) Configured() bool {
	return c.baseURL != "" && c.apiKey != "" && c.model != ""
}

// Message is one chat turn.
type Message struct {
	Role    string `json:"role"` // "system" | "user" | "assistant"
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends the messages and returns the assistant's reply text. temperature is
// kept low (0.2) for RCA: we want grounded, repeatable analysis, not creativity.
// Transient failures (network errors, 429, 5xx — e.g. a hosted model's brief "high demand"
// 503) are retried with backoff, since losing an RCA to a momentary spike isn't acceptable.
func (c *Client) Complete(ctx context.Context, messages []Message) (string, error) {
	body, err := json.Marshal(chatRequest{Model: c.model, Messages: messages, Temperature: 0.2})
	if err != nil {
		return "", err
	}

	const attempts = 3
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		out, retryable, err := c.tryComplete(ctx, body)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !retryable || attempt == attempts {
			break
		}
		// Backoff (2s, 4s), but never past the caller's deadline.
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt*2) * time.Second):
		}
	}
	return "", lastErr
}

// tryComplete makes one attempt; retryable reports whether a failure is worth retrying.
func (c *Client) tryComplete(ctx context.Context, body []byte) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", true, err // network/timeout — retryable
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return "", retryable, fmt.Errorf("llm http %d: %s", resp.StatusCode, string(raw))
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", false, fmt.Errorf("decode llm response: %w", err)
	}
	if parsed.Error != nil {
		return "", false, fmt.Errorf("llm error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", false, fmt.Errorf("llm returned no choices")
	}
	return parsed.Choices[0].Message.Content, false, nil
}
