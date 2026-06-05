package rca

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tomjga/OmniObserve/remediator/internal/corpus"
	"github.com/tomjga/OmniObserve/remediator/internal/evidence"
	"github.com/tomjga/OmniObserve/remediator/internal/llm"
)

func TestTerms_SplitsCamelCase(t *testing.T) {
	got := terms(Incident{AlertName: "ProductCatalogHighErrorRate", Service: "product-catalog"})
	joined := strings.Join(got, " ")
	for _, want := range []string{"Product", "Catalog", "High", "Error", "Rate", "product-catalog"} {
		if !strings.Contains(joined, want) {
			t.Errorf("terms %v missing %q", got, want)
		}
	}
}

func TestEnabled(t *testing.T) {
	if New(llm.New("", "", ""), nil, nil).Enabled() {
		t.Error("unconfigured LLM should make the copilot disabled")
	}
	if !New(llm.New("u", "m", "k"), nil, nil).Enabled() {
		t.Error("configured LLM should enable the copilot")
	}
}

func TestDraft_BuildsGroundedPromptAndReturnsRCA(t *testing.T) {
	// Mock Prometheus: return data only for gRPC queries.
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("query"), "rpc_server_duration") {
			_, _ = w.Write([]byte(`{"data":{"result":[{"value":[1,"0.5"]}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"result":[]}}`))
	}))
	defer prom.Close()

	// Mock LLM: capture the prompt it receives, return a canned RCA.
	var prompt string
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []llm.Message `json:"messages"`
		}
		_ = json.Unmarshal(raw, &req)
		for _, m := range req.Messages {
			prompt += m.Content + "\n"
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"## Summary\nflagd fault"}}]}`))
	}))
	defer llmSrv.Close()

	incidents := []corpus.Incident{{
		ID: "INC-2026-0007", Title: "Feature-flag remediation didn't reach services",
		Tags: []string{"flagd", "product-catalog"}, Services: []string{"product-catalog"},
		Body: "flagd served a seed-once copy.",
	}}

	cp := New(llm.New(llmSrv.URL, "m", "k"), evidence.NewPrometheus(prom.URL), incidents)
	out, err := cp.Draft(context.Background(), Incident{
		AlertName: "ProductCatalogHighErrorRate", Service: "product-catalog",
		Summary: "product-catalog gRPC error ratio above 5%",
		Action:  "disabled flagd flag productCatalogFailure",
	})
	if err != nil {
		t.Fatalf("draft error: %v", err)
	}
	if !strings.Contains(out, "Summary") {
		t.Errorf("RCA output missing expected content: %q", out)
	}

	// The prompt must include the evidence value and the retrieved precedent ID — proof
	// the RCA is grounded, not free-form.
	if !strings.Contains(prompt, "0.5") {
		t.Error("prompt did not include the Prometheus evidence")
	}
	if !strings.Contains(prompt, "INC-2026-0007") {
		t.Error("prompt did not include the retrieved prior incident")
	}
	if !strings.Contains(prompt, "disabled flagd flag productCatalogFailure") {
		t.Error("prompt did not include the action the remediator took")
	}
	// The system topology must be in the prompt so the model reasons about connectivity.
	if !strings.Contains(prompt, "System architecture") || !strings.Contains(prompt, "flagd feature flags") {
		t.Error("prompt did not include the system architecture / topology context")
	}
}
