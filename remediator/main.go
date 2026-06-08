// Command remediator is OmniObserve's control-loop service. It receives Alertmanager
// webhooks when an SLO burns, takes a bounded auditable action, verifies whether the
// signal improved, and optionally drafts an RCA.
package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"sync"
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

	"github.com/tomjga/OmniObserve/services/shared/telemetry"
)

// logger is the package-level structured logger, assigned in main (mirrors api-service).
var logger *zap.SugaredLogger

// version is injected at build time via -ldflags "-X main.version=<git version>".
var version = "dev"

// flagRemediator performs the bounded action (disable a flagd flag). It is nil when no
// in-cluster Kubernetes config is available (e.g. local `go run`, tests), in which case
// the service stays observe-only — every action call site is nil-guarded.
var flagRemediator *FlagRemediator

// rcaQueue bounds asynchronous RCA drafting. It stays nil when the copilot is disabled.
var rcaQueue *RCAQueue

// noopGuard detects repeated idempotent "already safe" outcomes for the same incident.
// That pattern means the remediator has no useful state transition left to perform.
var noopGuard *NoopStormGuard

var faultCatalog *FaultCatalog

var autonomousStop bool

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
	// trail for every remediation decision.
	actionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "remediator_actions_total",
			Help: "Remediation actions attempted, by flag and outcome.",
		},
		[]string{"flag", "outcome"},
	)
	noopStormsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "remediator_noop_storms_total",
			Help: "Repeated already-safe remediations escalated as needing human attention.",
		},
		[]string{"flag"},
	)
	webhookDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "remediator_webhook_duration_seconds",
			Help:    "Alertmanager webhook handler duration.",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	prometheus.MustRegister(alertsReceived, actionsTotal, noopStormsTotal, webhookDuration)
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
	shutdownTracer, err := telemetry.InitTracer(context.Background(), "remediator", version)
	if err != nil {
		panic(err)
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	zapLogger, _ := zap.NewProduction()
	defer func() { _ = zapLogger.Sync() }()
	logger = zapLogger.Sugar()

	autonomousStop = os.Getenv("REMEDIATOR_STOP") == "true"
	if autonomousStop {
		logger.Warn("global remediation stop switch enabled; actions will not mutate state")
	}
	var ok bool
	autonomyMode, ok = parseAutonomyMode(envStr("REMEDIATOR_AUTONOMY_MODE", string(AutonomyAutoWithVerify)))
	if !ok {
		logger.Warnw("invalid autonomy mode; defaulting to auto-with-verify", "configured", os.Getenv("REMEDIATOR_AUTONOMY_MODE"))
	}
	noopGuard = NewNoopStormGuard(
		envInt("REMEDIATOR_NOOP_STORM_THRESHOLD", 3),
		time.Duration(envInt("REMEDIATOR_NOOP_STORM_WINDOW_SECONDS", 900))*time.Second,
	)
	approvalStore = NewApprovalStore()
	var catalogErr error
	faultCatalog, catalogErr = LoadFaultCatalog(envStr("REMEDIATION_CATALOG_PATH", "/etc/remediator/fault-catalog.json"))
	if catalogErr != nil {
		logger.Warnw("using fallback fault catalog", "error", catalogErr)
	}
	logger.Infow("fault catalog ready", "entries", len(faultCatalog.Faults), "global_autonomy", string(autonomyMode))
	verifier = initVerifier()
	if verifier != nil {
		logger.Infow("post-action verifier ready", "delay", verifier.delay.String())
	}
	flagRemediator = initRemediator()
	copilot, publisher = initCopilot()
	if copilot != nil && copilot.Enabled() {
		workers := envInt("RCA_WORKERS", 2)
		rcaQueue = NewRCAQueue(workers, envInt("RCA_QUEUE_DEPTH", 20))
		rcaQueue.Start(workers)
		logger.Infow("rca queue ready", "workers", workers, "depth", envInt("RCA_QUEUE_DEPTH", 20))
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware("remediator")) // one span per request
	router.Use(timeoutMiddleware(30 * time.Second))
	router.Use(corsMiddleware())
	registerRoutes(router)

	mode := "observe-only"
	if flagRemediator != nil {
		mode = "active"
	}
	logger.Infow("remediator starting", "version", version, "mode", mode)
	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}

func registerRoutes(router *gin.Engine) {
	router.POST("/webhook", webhookAuthMiddleware(), webhookHandler) // Alertmanager posts here
	router.GET("/approvals", approvalAuthMiddleware(), listApprovalsHandler)
	router.POST("/approvals/:id/approve", approvalAuthMiddleware(), approveApprovalHandler)
	router.POST("/approvals/:id/deny", approvalAuthMiddleware(), denyApprovalHandler)
	router.GET("/healthz", healthHandler)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// webhookHandler parses an Alertmanager webhook and, for now, only records what it sees.
// Each firing alert becomes a log line, a counter increment, and a span event — the
// raw material the action step (disable the offending feature flag) and the RCA copilot
// will build on.
func webhookHandler(c *gin.Context) {
	start := time.Now()
	defer func() { webhookDuration.Observe(time.Since(start).Seconds()) }()

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
	if alert.Status != "firing" {
		return
	}
	policy, ok := faultCatalog.Lookup(alert)
	if !ok || policy.Action.Type != "disable-flag" || policy.Action.Flag == "" {
		recordAction("none", OutcomeUnsupported, span)
		logger.Infow("remediation unsupported",
			"reason", "no_catalog_policy",
			"incident_key", alert.incidentKey(),
			"alertname", alert.alertName())
		return
	}
	flag := policy.Action.Flag
	mode := effectiveAutonomy(autonomyMode, policy)
	if annotated := alert.remediationFlag(); annotated != "" && annotated != flag {
		logger.Warnw("alert remediation_flag ignored; catalog is authoritative",
			"incident_key", alert.incidentKey(),
			"annotated_flag", annotated,
			"catalog_flag", flag)
	}
	if autonomousStop {
		recordAction(flag, OutcomeNeedsHuman, span)
		logger.Warnw("global stop switch blocked remediation",
			"flag", flag,
			"incident_key", alert.incidentKey())
		return
	}
	switch mode {
	case AutonomyObserve:
		recordAction(flag, OutcomeNeedsHuman, span)
		logger.Infow("autonomy observe: action suppressed",
			"flag", flag,
			"incident_key", alert.incidentKey())
		return
	case AutonomySuggest:
		recordAction(flag, OutcomeNeedsHuman, span)
		logger.Infow("autonomy suggest: drafting recommendation without mutation",
			"flag", flag,
			"incident_key", alert.incidentKey())
		if rcaQueue != nil {
			rcaQueue.EnqueueWithVerification(alert,
				"suggested disabling flagd flag "+flag+"; no mutation performed (autonomy mode suggest)",
				"not_applicable")
		}
		return
	case AutonomyApproval:
		approval := queueApproval(alert, flag)
		recordAction(flag, OutcomeNeedsHuman, span)
		logger.Warnw("autonomy approval: human approval required before mutation",
			"approval_id", approval.ID,
			"flag", flag,
			"incident_key", alert.incidentKey())
		if rcaQueue != nil {
			rcaQueue.EnqueueWithVerification(alert,
				"approval required before disabling flagd flag "+flag+"; no mutation performed",
				"not_applicable")
		}
		return
	}
	if flagRemediator == nil {
		recordAction(flag, OutcomeUnsupported, span)
		logger.Infow("remediation unsupported",
			"reason", "remediator_inactive",
			"flag", flag,
			"incident_key", alert.incidentKey())
		return
	}

	var before []ObservedMetric
	if verifier != nil {
		before = verifier.Gather(ctx, alert.serviceName())
	}

	outcome, err := flagRemediator.DisableFlag(ctx, flag, alert.incidentKey())
	if err != nil {
		outcome = OutcomeFailed
		logger.Errorw("remediation failed", "flag", flag, "incident_key", alert.incidentKey(), "error", err)
	} else {
		if outcome == OutcomeAlreadySafe && noopGuard != nil && noopGuard.Record(flag, alert.incidentKey()) {
			outcome = OutcomeNeedsHuman
			noopStormsTotal.WithLabelValues(flag).Inc()
			logger.Warnw("no-op remediation storm; escalating",
				"flag", flag,
				"incident_key", alert.incidentKey(),
				"threshold", noopGuard.threshold,
				"window", noopGuard.window.String())
		}
		logger.Infow("remediation",
			"flag", flag, "outcome", string(outcome), "incident_key", alert.incidentKey())
	}
	recordAction(flag, outcome, span)

	// When we actually disabled a flag (once per incident — repeats hit cooldown), draft
	// a grounded RCA in the bounded queue. The LLM call never blocks the webhook, and
	// incident storms cannot create unbounded goroutines.
	if outcome == OutcomeHealed {
		if mode == AutonomyAutoWithVerify {
			verifyAndDraftAfterAction(alert, flag, before)
		} else {
			verifyAfterAction(alert, flag, before)
			if rcaQueue != nil {
				rcaQueue.EnqueueWithVerification(alert, "disabled flagd flag "+flag, "pending")
			}
		}
	}
}

func recordAction(flag string, outcome Outcome, span trace.Span) {
	actionsTotal.WithLabelValues(flag, string(outcome)).Inc()
	span.AddEvent("remediation", trace.WithAttributes(
		attribute.String("flag", flag),
		attribute.String("outcome", string(outcome)),
	))
}

// NoopStormGuard tracks repeated idempotent outcomes in-memory. Persistence is not needed
// for the guard: persisted cooldowns protect mutation; this only quiets alert noise.
type NoopStormGuard struct {
	mu        sync.Mutex
	seen      map[string][]time.Time
	threshold int
	window    time.Duration
}

func NewNoopStormGuard(threshold int, window time.Duration) *NoopStormGuard {
	if threshold < 1 {
		threshold = 1
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &NoopStormGuard{seen: map[string][]time.Time{}, threshold: threshold, window: window}
}

func (g *NoopStormGuard) Record(flag, incidentKey string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	key := flag + "|" + incidentKey
	cutoff := now.Add(-g.window)
	var recent []time.Time
	for _, at := range g.seen[key] {
		if at.After(cutoff) {
			recent = append(recent, at)
		}
	}
	recent = append(recent, now)
	g.seen[key] = recent
	return len(recent) > g.threshold
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

func webhookAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := os.Getenv("WEBHOOK_BEARER_TOKEN")
		if token == "" {
			c.Next()
			return
		}
		if c.GetHeader("Authorization") != "Bearer "+token {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized webhook"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func approvalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := os.Getenv("APPROVAL_BEARER_TOKEN")
		if token == "" {
			c.Next()
			return
		}
		if c.GetHeader("Authorization") != "Bearer "+token {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized approval request"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
