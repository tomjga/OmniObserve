// Command remediator is OmniObserve's control-loop service. It receives Alertmanager
// webhooks when an SLO burns and (in later steps) takes a bounded, auditable action to
// stop the bleeding. This first cut is OBSERVE-ONLY: it parses alerts, logs them, and
// counts them — no mutation yet. That lets us wire and validate the alert path end to
// end before giving the loop any power.
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// logger is the package-level structured logger, assigned in main (mirrors api-service).
var logger *zap.SugaredLogger

// version is injected at build time via -ldflags "-X main.version=<git version>".
var version = "dev"

// alertsReceived counts alerts pulled out of webhooks, by rule and firing/resolved
// status. This is the audit trail's first metric: "what did the remediator see?"
var alertsReceived = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "remediator_alerts_received_total",
		Help: "Alerts received from Alertmanager, by alertname and status.",
	},
	[]string{"alertname", "status"},
)

func init() {
	prometheus.MustRegister(alertsReceived)
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

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware("remediator")) // one span per request
	router.Use(timeoutMiddleware(30 * time.Second))

	router.POST("/webhook", webhookHandler) // Alertmanager posts here
	router.GET("/healthz", healthHandler)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	logger.Infow("remediator starting", "version", version, "mode", "observe-only")
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
	}

	c.JSON(http.StatusOK, gin.H{"received": len(payload.Alerts)})
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
