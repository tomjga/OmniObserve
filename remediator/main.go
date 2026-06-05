// Command remediator is OmniObserve's control-loop service. It receives Alertmanager
// webhooks when an SLO burns and (in later steps) takes a bounded, auditable action to
// stop the bleeding. This first cut is OBSERVE-ONLY: it parses alerts, logs them, and
// counts them — no mutation yet. That lets us wire and validate the alert path end to
// end before giving the loop any power.
package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// logger is the package-level structured logger, assigned in main (mirrors api-service).
var logger *zap.SugaredLogger

// version is injected at build time via -ldflags "-X main.version=<git version>".
var version = "dev"

// flagRemediator performs the bounded action (disable a flagd flag). It is nil when no
// in-cluster Kubernetes config is available (e.g. local `go run`, tests), in which case
// the service stays observe-only — every action call site is nil-guarded.
var flagRemediator *FlagRemediator

var (
	// alertsReceived: "what did the remediator see?"
	alertsReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "remediator_alerts_received_total",
			Help: "Alerts received from Alertmanager, by alertname and status.",
		},
		[]string{"alertname", "status"},
	)
	// actionsTotal: "what did the remediator do, and how did it turn out?" — the audit
	// trail for every remediation decision (disabled/already_off/dry_run/cooldown/error).
	actionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "remediator_actions_total",
			Help: "Remediation actions attempted, by flag and outcome.",
		},
		[]string{"flag", "outcome"},
	)
)

func init() {
	prometheus.MustRegister(alertsReceived, actionsTotal)
}

// initRemediator builds the flagd action from env config, or returns nil (observe-only)
// when there's no in-cluster Kubernetes API to act against.
func initRemediator() *FlagRemediator {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		logger.Warnw("no in-cluster config; running observe-only (no actions)", "error", err)
		return nil
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Warnw("could not build kubernetes client; running observe-only", "error", err)
		return nil
	}
	cooldown := time.Duration(envInt("REMEDIATOR_COOLDOWN_SECONDS", 300)) * time.Second
	dryRun := os.Getenv("REMEDIATOR_DRY_RUN") == "true"
	r := NewFlagRemediator(clientset,
		envStr("FLAGD_NAMESPACE", "otel-demo"),
		envStr("FLAGD_CONFIGMAP", "flagd-config"),
		envStr("FLAGD_CONFIG_KEY", "demo.flagd.json"),
		dryRun, cooldown,
	)
	logger.Infow("flag remediator ready", "dryRun", dryRun, "cooldown", cooldown.String())
	return r
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	shutdownTracer, err := initTracer()
	if err != nil {
		panic(err)
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	zapLogger, _ := zap.NewProduction()
	defer func() { _ = zapLogger.Sync() }()
	logger = zapLogger.Sugar()

	flagRemediator = initRemediator()

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware("remediator")) // one span per request
	router.Use(timeoutMiddleware(30 * time.Second))

	router.POST("/webhook", webhookHandler) // Alertmanager posts here
	router.GET("/healthz", healthHandler)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	mode := "observe-only"
	if flagRemediator != nil {
		mode = "active"
	}
	logger.Infow("remediator starting", "version", version, "mode", mode)
	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}

// webhookHandler parses an Alertmanager webhook and, for now, only records what it sees.
// Each firing alert becomes a log line, a counter increment, and a span event — the
// raw material the action step (disable the offending feature flag) and the RCA copilot
// will build on.
func webhookHandler(c *gin.Context) {
	var payload AlertmanagerWebhook
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Warnw("webhook: bad payload", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook payload"})
		return
	}

	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		attribute.String("alertmanager.status", payload.Status),
		attribute.Int("alertmanager.alerts", len(payload.Alerts)),
	)

	for _, alert := range payload.Alerts {
		alertsReceived.WithLabelValues(alert.alertName(), alert.Status).Inc()
		logger.Infow("alert received",
			"alertname", alert.alertName(),
			"status", alert.Status,
			"incident_key", alert.incidentKey(),
			"severity", alert.Labels["severity"],
			"summary", alert.Annotations["summary"],
		)
		span.AddEvent("alert", trace.WithAttributes(
			attribute.String("alertname", alert.alertName()),
			attribute.String("status", alert.Status),
			attribute.String("incident_key", alert.incidentKey()),
		))
		remediate(c.Request.Context(), span, alert)
	}

	c.JSON(http.StatusOK, gin.H{"received": len(payload.Alerts)})
}

// remediate runs the bounded action for one alert: only firing alerts that explicitly
// name a flag are acted on, and only when the remediator is active (has a cluster).
func remediate(ctx context.Context, span trace.Span, alert Alert) {
	flag := alert.remediationFlag()
	if alert.Status != "firing" || flag == "" || flagRemediator == nil {
		return
	}

	outcome, err := flagRemediator.DisableFlag(ctx, flag, alert.incidentKey())
	result := string(outcome)
	if err != nil {
		result = "error"
		logger.Errorw("remediation failed", "flag", flag, "incident_key", alert.incidentKey(), "error", err)
	} else {
		logger.Infow("remediation",
			"flag", flag, "outcome", result, "incident_key", alert.incidentKey())
	}
	actionsTotal.WithLabelValues(flag, result).Inc()
	span.AddEvent("remediation", trace.WithAttributes(
		attribute.String("flag", flag),
		attribute.String("outcome", result),
	))
}

// healthHandler reports liveness and the build version (same contract as api-service).
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version})
}

// timeoutMiddleware bounds handler execution so a slow downstream can't pile up requests.
func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
