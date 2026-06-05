// Package rca is the RCA copilot: given an incident, it gathers evidence from Prometheus,
// retrieves relevant precedent from the incident corpus, and asks a vendor-agnostic LLM to
// draft a structured root-cause analysis grounded strictly in that material. The grounding
// is the point — this is not a chatbot, it's precedent-aware incident analysis that gets
// sharper as the corpus grows.
package rca

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/tomjga/OmniObserve/remediator/internal/corpus"
	"github.com/tomjga/OmniObserve/remediator/internal/evidence"
	"github.com/tomjga/OmniObserve/remediator/internal/llm"
)

// Incident is the context the remediator hands the copilot when an alert fires.
type Incident struct {
	AlertName   string
	Service     string
	Summary     string
	IncidentKey string
	Action      string // what the remediator did, e.g. "disabled flagd flag productCatalogFailure"
	StartsAt    time.Time
}

// Copilot drafts RCAs. Construct with New.
type Copilot struct {
	llm       *llm.Client
	prom      *evidence.Prometheus
	incidents []corpus.Incident
}

func New(client *llm.Client, prom *evidence.Prometheus, incidents []corpus.Incident) *Copilot {
	return &Copilot{llm: client, prom: prom, incidents: incidents}
}

// Enabled reports whether the copilot can draft (i.e. the LLM is configured).
func (c *Copilot) Enabled() bool { return c.llm != nil && c.llm.Configured() }

// Draft produces a markdown RCA for the incident. It is best-effort about evidence and
// precedent (missing either just means a thinner prompt), but requires the LLM to answer.
func (c *Copilot) Draft(ctx context.Context, inc Incident) (string, error) {
	var metrics []evidence.Metric
	if c.prom != nil {
		metrics = c.prom.Gather(ctx, inc.Service)
	}
	precedent := corpus.Retrieve(c.incidents, terms(inc), 3)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt(inc, metrics, precedent)},
	}
	return c.llm.Complete(ctx, messages)
}

const systemPrompt = `You are an SRE incident-analysis assistant for the OmniObserve platform.
Write a concise root-cause analysis grounded STRICTLY in the evidence and prior incidents
provided. Do not invent metrics, logs, or causes that are not supported by the material.
If the evidence is thin, say so. Prefer the explanation most consistent with the cited
prior incidents. Output markdown with exactly these sections:
## Summary
## Likely root cause
## Evidence considered
## Recommended follow-up
## Related prior incidents (cite their IDs)`

func userPrompt(inc Incident, metrics []evidence.Metric, precedent []corpus.Incident) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Incident\n")
	fmt.Fprintf(&b, "- Alert: %s\n- Service: %s\n- Summary: %s\n", inc.AlertName, inc.Service, inc.Summary)
	if !inc.StartsAt.IsZero() {
		fmt.Fprintf(&b, "- Started: %s\n", inc.StartsAt.UTC().Format(time.RFC3339))
	}
	if inc.Action != "" {
		fmt.Fprintf(&b, "- Automated action already taken by the remediator: %s\n", inc.Action)
	}

	b.WriteString("\n# Evidence (Prometheus)\n")
	if len(metrics) == 0 {
		b.WriteString("(no metrics returned for this service)\n")
	}
	for _, m := range metrics {
		fmt.Fprintf(&b, "- %s: %s\n", m.Name, m.Value)
	}

	b.WriteString("\n# Prior incidents (most relevant first)\n")
	if len(precedent) == 0 {
		b.WriteString("(no closely related prior incidents found)\n")
	}
	for _, p := range precedent {
		fmt.Fprintf(&b, "## %s — %s\nTags: %s\n%s\n\n", p.ID, p.Title, strings.Join(p.Tags, ", "), p.Body)
	}
	return b.String()
}

var camel = regexp.MustCompile(`[A-Z][a-z]+|[A-Z]+(?:[A-Z][a-z])|[a-z]+|[0-9]+`)

// terms derives retrieval keywords from the incident: the camelCase-split alert name,
// the service, and the summary words. This is what the corpus is searched on.
func terms(inc Incident) []string {
	var t []string
	t = append(t, camel.FindAllString(inc.AlertName, -1)...)
	t = append(t, strings.Fields(inc.Summary)...)
	if inc.Service != "" {
		t = append(t, inc.Service)
	}
	return t
}
