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
	// SystemContext describes how the monitored system is wired (topology + signal flow), so
	// the LLM can reason about cause and blast radius instead of guessing. Defaults to the
	// OmniObserve topology; override it (e.g. via the SYSTEM_CONTEXT env) to point the same
	// copilot at a different monitored system.
	SystemContext string
}

func New(client *llm.Client, prom *evidence.Prometheus, incidents []corpus.Incident) *Copilot {
	return &Copilot{llm: client, prom: prom, incidents: incidents, SystemContext: defaultSystemContext}
}

// defaultSystemContext is the OmniObserve topology. In this environment faults are injected
// for testing via flagd feature flags, so we say so explicitly — it keeps the model from
// over-diagnosing a synthetic fault as a code defect.
const defaultSystemContext = `OmniObserve is a local Kubernetes observability + auto-remediation platform.
Topology and signal flow:
- Workloads: the OpenTelemetry Demo microservices (product-catalog, frontend, ...) in the
  'otel-demo' namespace. Faults are injected FOR TESTING via flagd feature flags (e.g.
  productCatalogFailure makes product-catalog throw gRPC errors) — so here a firing alert
  usually traces back to an enabled fault flag, not a code regression.
- Telemetry: services export OTLP to the OTel Collector, which fans metrics out to Prometheus
  and traces to Tempo. Prometheus recording/alert rules encode the SLOs.
- Alerting: when an SLO burns, Prometheus -> Alertmanager -> the 'remediator' service webhook.
- Remediation: the remediator takes ONE bounded, reversible action — disabling the flagd flag
  named in the alert's remediation_flag annotation (cooldown- and dry-run-guarded). flagd
  watches its ConfigMap, so the change reloads live and consuming services recover.
Use this topology to reason about likely cause and blast radius.`

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

	user := userPrompt(inc, metrics, precedent)
	if c.SystemContext != "" {
		user = "# System architecture\n" + c.SystemContext + "\n\n" + user
	}
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: user},
	}
	return c.llm.Complete(ctx, messages)
}

const systemPrompt = `You are an SRE incident-analysis assistant for the OmniObserve platform.
Write a concise root-cause analysis grounded STRICTLY in the system architecture, evidence,
and prior incidents provided. Do not invent metrics, logs, or causes not supported by the
material. If the evidence is thin, say so. Prefer the explanation most consistent with the
described topology and the cited prior incidents. Output markdown with exactly these sections:
## Summary
## Likely root cause
## Evidence considered
## Proposed remediation
## Recommended follow-up
## Related prior incidents (cite their IDs)

For "Proposed remediation", give the most direct fix and state plainly whether the trigger is
a test-injected feature flag (the common case here — see the architecture) or a genuine code/
config defect; if it is a real defect, describe the concrete change that would resolve it.`

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
