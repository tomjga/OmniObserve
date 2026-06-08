package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tomjga/OmniObserve/remediator/internal/evidence"
)

type ObservedMetric = evidence.Metric

type Verifier struct {
	prom          *evidence.Prometheus
	delay         time.Duration
	sampleTimeout time.Duration
}

var verifier *Verifier

var actionVerificationsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "remediator_action_verifications_total",
		Help: "Post-action verification results, by flag and result.",
	},
	[]string{"flag", "result"},
)

func init() { prometheus.MustRegister(actionVerificationsTotal) }

func initVerifier() *Verifier {
	u := os.Getenv("PROMETHEUS_URL")
	if u == "" {
		return nil
	}
	return &Verifier{
		prom:          evidence.NewPrometheus(u),
		delay:         time.Duration(envInt("REMEDIATOR_VERIFY_DELAY_SECONDS", 30)) * time.Second,
		sampleTimeout: time.Duration(envInt("REMEDIATOR_VERIFY_SAMPLE_TIMEOUT_SECONDS", 3)) * time.Second,
	}
}

func (v *Verifier) Gather(ctx context.Context, service string) []ObservedMetric {
	if v == nil || v.prom == nil || service == "" {
		return nil
	}
	if v.sampleTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, v.sampleTimeout)
		defer cancel()
	}
	return v.prom.Gather(ctx, service)
}

func verifyAfterAction(alert Alert, flag string, before []ObservedMetric) {
	if verifier == nil {
		return
	}
	go verifier.Verify(context.Background(), alert, flag, before)
}

func (v *Verifier) Verify(ctx context.Context, alert Alert, flag string, before []ObservedMetric) {
	_ = v.VerifyResult(ctx, alert, flag, before)
}

func (v *Verifier) VerifyResult(ctx context.Context, alert Alert, flag string, before []ObservedMetric) string {
	if v == nil {
		return "not_configured"
	}
	if v.delay > 0 {
		select {
		case <-time.After(v.delay):
		case <-ctx.Done():
			return "cancelled"
		}
	}
	after := v.Gather(ctx, alert.serviceName())
	result := verificationResult(before, after)
	actionVerificationsTotal.WithLabelValues(flag, result).Inc()
	if logger != nil {
		logger.Infow("post-action verification",
			"flag", flag,
			"incident_key", alert.incidentKey(),
			"result", result,
			"before", metricSummary(before),
			"after", metricSummary(after))
	}
	return result
}

func verifyAndDraftAfterAction(alert Alert, flag string, before []ObservedMetric) {
	if rcaQueue == nil {
		verifyAfterAction(alert, flag, before)
		return
	}
	if verifier == nil {
		rcaQueue.EnqueueWithVerification(alert, "disabled flagd flag "+flag, "not_configured")
		return
	}
	go func() {
		result := verifier.VerifyResult(context.Background(), alert, flag, before)
		rcaQueue.EnqueueWithVerification(alert, "disabled flagd flag "+flag, result)
	}()
}

func verificationResult(before, after []ObservedMetric) string {
	beforeRatio, hasBefore := firstRatio(before)
	afterRatio, hasAfter := firstRatio(after)
	if !hasBefore {
		return "no_baseline"
	}
	if !hasAfter {
		return "no_after_data"
	}
	if afterRatio < beforeRatio {
		return "improved"
	}
	return "not_improved"
}

func firstRatio(metrics []ObservedMetric) (float64, bool) {
	for _, m := range metrics {
		name := strings.ToLower(m.Name)
		if !strings.Contains(name, "ratio") {
			continue
		}
		v, err := strconv.ParseFloat(m.Value, 64)
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

func metricSummary(metrics []ObservedMetric) string {
	var parts []string
	for _, m := range metrics {
		parts = append(parts, m.Name+"="+m.Value)
	}
	return strings.Join(parts, "; ")
}
