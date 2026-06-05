// Package evidence gathers quantitative signals about an incident from Prometheus, so the
// RCA copilot reasons over real numbers (error ratio, request rate) rather than just the
// alert text. Queries are templated on the alerting service's job label and cover both
// gRPC (rpc_server_*) and HTTP (http_requests_total) services; only those returning data
// are reported, so the same code works across the polyglot demo.
package evidence

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Prometheus is a thin instant-query client.
type Prometheus struct {
	baseURL string
	http    *http.Client
}

func NewPrometheus(baseURL string) *Prometheus {
	return &Prometheus{baseURL: baseURL, http: &http.Client{Timeout: 10 * time.Second}}
}

// Metric is one named evidence value.
type Metric struct {
	Name  string
	Value string
}

// query templates: {svc} is replaced with the alerting service's job label.
var queries = []struct{ name, expr string }{
	{"gRPC error ratio (5m)", `(sum(rate(rpc_server_duration_milliseconds_count{job="{svc}",rpc_grpc_status_code!="0"}[5m]))/clamp_min(sum(rate(rpc_server_duration_milliseconds_count{job="{svc}"}[5m])),1))`},
	{"gRPC request rate /s (5m)", `sum(rate(rpc_server_duration_milliseconds_count{job="{svc}"}[5m]))`},
	{"HTTP 5xx ratio (5m)", `(sum(rate(http_requests_total{job="{svc}",code=~"5.."}[5m]))/clamp_min(sum(rate(http_requests_total{job="{svc}"}[5m])),1))`},
	{"HTTP request rate /s (5m)", `sum(rate(http_requests_total{job="{svc}"}[5m]))`},
}

// Gather runs the templated queries for service and returns those that produced a value.
// Errors on individual queries are skipped (best-effort evidence), not fatal.
func (p *Prometheus) Gather(ctx context.Context, service string) []Metric {
	var out []Metric
	for _, q := range queries {
		expr := strings.ReplaceAll(q.expr, "{svc}", service)
		if v, ok := p.instant(ctx, expr); ok {
			out = append(out, Metric{Name: q.name, Value: v})
		}
	}
	return out
}

type queryResponse struct {
	Data struct {
		Result []struct {
			Value [2]any `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// instant runs an instant query and returns the first scalar value as a string, with ok
// false if the query failed or returned no series.
func (p *Prometheus) instant(ctx context.Context, expr string) (string, bool) {
	u := p.baseURL + "/api/v1/query?query=" + url.QueryEscape(expr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", false
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return "", false
	}
	raw, _ := io.ReadAll(resp.Body)
	var parsed queryResponse
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Data.Result) == 0 {
		return "", false
	}
	if s, ok := parsed.Data.Result[0].Value[1].(string); ok {
		return s, true
	}
	return fmt.Sprintf("%v", parsed.Data.Result[0].Value[1]), true
}
