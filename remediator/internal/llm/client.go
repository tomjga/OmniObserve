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
func (c *Client) Complete(ctx context.Context, messages []Message) (string, error) {
	body, err := json.Marshal(chatRequest{Model: c.model, Messages: messages, Temperature: 0.2})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm http %d: %s", resp.StatusCode, string(raw))
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("llm error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
