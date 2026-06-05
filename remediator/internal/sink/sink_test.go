package sink

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigured(t *testing.T) {
	if (Grafana{URL: "u"}).Configured() {
		t.Error("Grafana without token should be unconfigured")
	}
	if !(GitHubIssue{Repo: "o/r", Token: "t"}).Configured() {
		t.Error("GitHubIssue with repo+token should be configured")
	}
	if (GitHubCorpus{Repo: "o/r", Token: "t"}).Configured() {
		t.Error("GitHubCorpus without branch should be unconfigured")
	}
}

func TestGrafana_PostsAnnotation(t *testing.T) {
	var gotAuth string
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &payload)
		if r.URL.Path != "/api/annotations" {
			t.Errorf("path = %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	g := Grafana{URL: srv.URL, Token: "tok", HTTP: srv.Client()}
	if err := g.Publish(context.Background(), RCA{Title: "T", Body: "B", Service: "svc"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %q", gotAuth)
	}
	if !strings.Contains(payload["text"].(string), "T") {
		t.Errorf("annotation text missing title: %v", payload["text"])
	}
}

func TestGitHubCorpus_CommitsToDraftsBranch(t *testing.T) {
	var gotPath string
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &payload)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	// Point the client at the test server by overriding transport via a custom client
	// is overkill; instead assert the request shape through a RoundTripper.
	gc := GitHubCorpus{Repo: "o/r", Token: "t", Branch: "rca-drafts", HTTP: &http.Client{
		Transport: rewriteHost(srv.URL),
	}}
	if err := gc.Publish(context.Background(), RCA{Title: "T", Body: "# body", Slug: "svcfail"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.Contains(gotPath, "/repos/o/r/contents/incidents/") || !strings.HasSuffix(gotPath, "-svcfail-rca.md") {
		t.Errorf("unexpected path %q", gotPath)
	}
	if payload["branch"] != "rca-drafts" {
		t.Errorf("branch = %v, want rca-drafts", payload["branch"])
	}
	if _, ok := payload["content"].(string); !ok {
		t.Error("content (base64) missing")
	}
}

func TestPublisher_SkipsUnconfiguredCollectsResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	p := NewPublisher(
		Grafana{URL: srv.URL, Token: "t", HTTP: srv.Client()}, // configured
		GitHubIssue{},  // unconfigured -> skipped
		GitHubCorpus{}, // unconfigured -> skipped
	)
	results := p.Publish(context.Background(), RCA{Title: "T", Body: "B"})
	if len(results) != 1 || results[0].Sink != "grafana" || results[0].Error != nil {
		t.Errorf("expected only grafana to run cleanly, got %+v", results)
	}
}

// rewriteHost sends api.github.com requests to the test server instead.
type hostRewriter struct{ target string }

func rewriteHost(target string) http.RoundTripper {
	return hostRewriter{target: strings.TrimPrefix(target, "http://")}
}

func (h hostRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = h.target
	return http.DefaultTransport.RoundTrip(req)
}
