package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigured(t *testing.T) {
	if New("", "m", "k").Configured() {
		t.Error("empty baseURL should be unconfigured")
	}
	if New("u", "m", "").Configured() {
		t.Error("empty apiKey should be unconfigured")
	}
	if !New("u", "m", "k").Configured() {
		t.Error("fully set should be configured")
	}
}

func TestComplete_SendsRequestAndParsesReply(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody chatRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"root cause: X"}}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "test-model", "secret")
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "why?"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out != "root cause: X" {
		t.Errorf("content = %q, want 'root cause: X'", out)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("auth header = %q, want 'Bearer secret'", gotAuth)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", gotPath)
	}
	if gotBody.Model != "test-model" || len(gotBody.Messages) != 1 {
		t.Errorf("request body wrong: %+v", gotBody)
	}
}

func TestComplete_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, "m", "k").Complete(context.Background(), nil)
	if err == nil {
		t.Fatal("expected an error on HTTP 429")
	}
}

func TestComplete_NoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "m", "k").Complete(context.Background(), nil); err == nil {
		t.Fatal("expected an error when no choices are returned")
	}
}
