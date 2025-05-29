package main

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// Metrics setup
var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"code", "method", "endpoint"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Request duration distribution",
			Buckets: []float64{0.1, 0.3, 0.5, 1, 3},
		},
		[]string{"endpoint"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration)
}

// prometheusMiddleware records metrics for all requests
func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		statusCode := c.Writer.Status()
		requestsTotal.WithLabelValues(
			http.StatusText(statusCode),
			c.Request.Method,
			c.FullPath(),
		).Inc()

		requestDuration.WithLabelValues(c.FullPath()).Observe(duration.Seconds())
	}
}

func main() {
	// Initialize Datadog tracer
	tracer.Start(
		tracer.WithService("kpi-service"),
		tracer.WithEnv("production"),
	)
	defer tracer.Stop()

	router := gin.Default()
	
	// Add middleware
	router.Use(prometheusMiddleware())
	router.Use(timeoutMiddleware(30 * time.Second)) // Add timeout middleware

	// KPI testing endpoints with configurable parameters
	router.GET("/kpi/availability", availabilityHandler)
	router.GET("/kpi/performance", performanceHandler)
	router.GET("/kpi/errors", errorRateHandler)
	router.GET("/benchmark", benchmarkHandler)
	
	// Health check endpoint
	router.GET("/healthz", healthHandler)
	
	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Start server with error handling
	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}

// timeoutMiddleware adds context timeout to handlers
func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// healthHandler implements a simple health check
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "1.0.0",
	})
}

func availabilityHandler(c *gin.Context) {
	// Get success rate from query parameter (default 99.9%)
	successRate := 99.9
	if rateParam := c.Query("success_rate"); rateParam != "" {
		if rate, err := strconv.ParseFloat(rateParam, 64); err == nil && rate >= 0 && rate <= 100 {
			successRate = rate
		}
	}

	// Simulate availability with configurable success rate
	if rand.Float64()*100 < successRate {
		c.JSON(http.StatusOK, gin.H{
			"status":      "available",
			"success_rate": successRate,
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":      "unavailable",
			"success_rate": successRate,
		})
	}
}

func performanceHandler(c *gin.Context) {
	// Get max delay from query parameter (default 500ms)
	maxDelay := 500
	if delayParam := c.Query("max_delay"); delayParam != "" {
		if delay, err := strconv.Atoi(delayParam); err == nil && delay > 0 {
			maxDelay = delay
		}
	}

	// Simulate latency with configurable max delay
	delay := time.Duration(rand.Intn(maxDelay)) * time.Millisecond
	time.Sleep(delay)
	c.JSON(http.StatusOK, gin.H{
		"latency_ms": delay.Milliseconds(),
		"max_delay":  maxDelay,
	})
}

func errorRateHandler(c *gin.Context) {
	// Get error rate from query parameter (default 5%)
	errorRate := 5.0
	if rateParam := c.Query("error_rate"); rateParam != "" {
		if rate, err := strconv.ParseFloat(rateParam, 64); err == nil && rate >= 0 && rate <= 100 {
			errorRate = rate
		}
	}

	// Simulate errors with configurable error rate
	if rand.Float64()*100 < errorRate {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "simulated_error",
			"error_rate": errorRate,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status":     "success",
			"error_rate": errorRate,
		})
	}
}

func benchmarkHandler(c *gin.Context) {
	start := time.Now()
	
	// Get delay from query parameter (default random up to 500ms)
	var delay time.Duration
	if delayParam := c.Query("delay"); delayParam != "" {
		if d, err := strconv.Atoi(delayParam); err == nil && d > 0 {
			delay = time.Duration(d) * time.Millisecond
		}
	}
	
	if delay == 0 {
		// If no fixed delay, use random with max from query
		maxDelay := 500
		if maxParam := c.Query("max_delay"); maxParam != "" {
			if m, err := strconv.Atoi(maxParam); err == nil && m > 0 {
				maxDelay = m
			}
		}
		delay = time.Duration(rand.Intn(maxDelay)) * time.Millisecond
	}
	
	time.Sleep(delay)
	latency := time.Since(start)
	
	// Track latency in Datadog
	if span, ok := tracer.SpanFromContext(c.Request.Context()); ok {
		span.SetTag("latency_ms", latency.Milliseconds())
	}
	
	c.JSON(http.StatusOK, gin.H{
		"latency_ms": latency.Milliseconds(),
		"delay_ms":   delay.Milliseconds(),
	})
}