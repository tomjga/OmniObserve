// Package sink publishes a drafted RCA to where humans (and the corpus) will see it:
// a Grafana annotation on the incident window, a GitHub issue for triage, and a committed
// RCA file that grows the corpus the copilot grounds future RCAs on. Every sink is
// config-gated (Configured) and best-effort — a missing token disables that sink, and a
// failure on one never blocks the others or the remediation itself.
package sink

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RCA is the drafted analysis plus the metadata the sinks need.
type RCA struct {
	Title    string
	Body     string // markdown
	Service  string
	Slug     string // filename-safe identifier, e.g. productcataloghigherrorrate
	Model    string // the LLM that drafted this, e.g. gemini-2.5-flash — surfaced as a tag/label
	StartsAt time.Time
}

func do(ctx context.Context, client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

// Grafana posts an annotation on the incident window.
type Grafana struct {
	URL   string // e.g. http://kps-grafana.monitoring
	Token string // service-account / API token
	HTTP  *http.Client
}

func (g Grafana) Configured() bool { return g.URL != "" && g.Token != "" }

func (g Grafana) Publish(ctx context.Context, r RCA) error {
	at := r.StartsAt
	if at.IsZero() {
		at = time.Now()
	}
	tags := []string{"omniobserve", "rca", r.Service}
	if r.Model != "" {
		tags = append(tags, "llm:"+r.Model)
	}
	body, _ := json.Marshal(map[string]any{
		"time": at.UnixMilli(),
		"tags": tags,
		"text": r.Title + "\n\n" + r.Body,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.URL+"/api/annotations", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.Token)
	return do(ctx, g.HTTP, req)
}

// GitHubIssue opens an issue with the RCA for human triage.
type GitHubIssue struct {
	Repo  string // owner/name
	Token string
	HTTP  *http.Client
}

func (g GitHubIssue) Configured() bool { return g.Repo != "" && g.Token != "" }

func (g GitHubIssue) Publish(ctx context.Context, r RCA) error {
	labels := []string{"rca", "automated"}
	if r.Model != "" {
		labels = append(labels, "llm:"+r.Model) // GitHub creates the label on first use
	}
	body, _ := json.Marshal(map[string]any{
		"title":  r.Title,
		"body":   r.Body,
		"labels": labels,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.github.com/repos/"+g.Repo+"/issues", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	return do(ctx, g.HTTP, req)
}

// GitHubCorpus commits the RCA as a new file on a drafts branch (never directly to main),
// so the corpus grows under human review. Path: incidents/<date>-<slug>-rca.md.
type GitHubCorpus struct {
	Repo   string // owner/name
	Token  string
	Branch string // e.g. rca-drafts
	HTTP   *http.Client
}

func (g GitHubCorpus) Configured() bool { return g.Repo != "" && g.Token != "" && g.Branch != "" }

func (g GitHubCorpus) Publish(ctx context.Context, r RCA) error {
	at := r.StartsAt
	if at.IsZero() {
		at = time.Now()
	}
	path := fmt.Sprintf("incidents/%s-%s-rca.md", at.UTC().Format("2006-01-02-1504"), r.Slug)
	body, _ := json.Marshal(map[string]any{
		"message": "rca(auto): " + r.Title,
		"content": base64.StdEncoding.EncodeToString([]byte(r.Body)),
		"branch":  g.Branch,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		"https://api.github.com/repos/"+g.Repo+"/contents/"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	return do(ctx, g.HTTP, req)
}

// Publisher fans an RCA out to all configured sinks, collecting (not short-circuiting on)
// errors so one bad sink doesn't suppress the others.
type Publisher struct {
	sinks map[string]interface {
		Configured() bool
		Publish(context.Context, RCA) error
	}
}

// NewPublisher registers the standard sinks; unconfigured ones are simply skipped.
func NewPublisher(g Grafana, gi GitHubIssue, gc GitHubCorpus) *Publisher {
	return &Publisher{sinks: map[string]interface {
		Configured() bool
		Publish(context.Context, RCA) error
	}{
		"grafana":       g,
		"github-issue":  gi,
		"github-corpus": gc,
	}}
}

// Result records, per sink, whether it ran and any error.
type Result struct {
	Sink  string
	Error error
}

// Publish sends to every configured sink and returns one Result per attempt.
func (p *Publisher) Publish(ctx context.Context, r RCA) []Result {
	var results []Result
	for name, s := range p.sinks {
		if !s.Configured() {
			continue
		}
		results = append(results, Result{Sink: name, Error: s.Publish(ctx, r)})
	}
	return results
}
