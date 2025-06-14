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
import (
	ginSwagger "github.com/swaggo/gin-swagger"
  	swaggerFiles "github.com/swaggo/files"
	_ "github.com/tomjga/OmniObserve/application/docs"
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



func main() {
	// Initialize Datadog tracer
	tracer.Start(
		tracer.WithService("kpi-service"),
		tracer.WithEnv("production"),
	)
	defer tracer.Stop()

	router := gin.Default()
	
	// Add middleware
	router.Use(metricsHandler())
	router.Use(timeoutMiddleware(30 * time.Second)) // Add timeout middleware

	// KPI testing endpoints with configurable parameters
	router.GET("/kpi/availability", availabilityHandler)
	router.POST("/kpi/availability", availabilityHandler)
	router.PUT("/kpi/availability", availabilityHandler)
	router.PATCH("/kpi/availability", availabilityHandler)
	
	router.GET("/kpi/performance", performanceHandler)
	router.POST("/kpi/performance", performanceHandler)
	router.PUT("/kpi/performance", performanceHandler)
	router.PATCH("/kpi/performance", performanceHandler)
	
	router.GET("/kpi/errors", errorRateHandler)
	router.POST("/kpi/errors", errorRateHandler)
	router.PUT("/kpi/errors", errorRateHandler)
	router.PATCH("/kpi/errors", errorRateHandler)
	
	router.GET("/benchmark", benchmarkHandler)
	router.POST("/benchmark", benchmarkHandler)
	router.PUT("/benchmark", benchmarkHandler)
	router.PATCH("/benchmark", benchmarkHandler)
	
	// Health check endpoint
	router.GET("/healthz", healthHandler)
	
	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
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


// Request structures
type AvailabilityRequest struct {
	SuccessRate float64 `json:"success_rate"`
}

type PerformanceRequest struct {
	MaxDelay int `json:"max_delay"`
}

type ErrorRateRequest struct {
	ErrorRate float64 `json:"error_rate"`
}

type BenchmarkRequest struct {
	Delay    *int `json:"delay"`    // Pointer to distinguish between 0 and not provided
	MaxDelay *int `json:"max_delay"`
}

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration)
}

// @Summary      Prometheus metrics
// @Description  Expose Prometheus-formatted metrics for this service
// @Tags         metrics
// @Produce      plain
// @Success      200  {string}  string  "metrics data"
// @Router       /metrics [get]
func metricsHandler() gin.HandlerFunc {
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

// @Summary      Health check
// @Description  Returns service health and version
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "service status"
// @Router       /healthz [get]
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "1.0.0",
	})
}

// @Summary      Check availability
// @Description  Simulate service availability with configurable success rate via query parameter
// @Tags         kpi
// @Produce      json
// @Param        success_rate  query     number                  false  "Success rate (0–100)"   default(99.9)
// @Success      200           {object}  map[string]interface{}  "available response"
// @Failure      503           {object}  map[string]interface{}  "unavailable response"
// @Router       /kpi/availability [get]

// @Summary      Check availability
// @Description  Simulate service availability with configurable success rate
// @Tags         kpi
// @Accept       json
// @Produce      json
// @Param        success_rate  query     number                 false  "Success rate (0–100)"           default(99.9)
// @Param        body          body      AvailabilityRequest     false  "Override success_rate via JSON"
// @Success      200           {object}  map[string]interface{} "available response"
// @Failure      503           {object}  map[string]interface{} "unavailable response"
// @Router       /kpi/availability [post]
// @Router       /kpi/availability [put]
// @Router       /kpi/availability [patch]
func availabilityHandler(c *gin.Context) {
	var req AvailabilityRequest
	successRate := 99.9

	// Handle input based on method
	if c.Request.Method == http.MethodGet {
		// GET method - use query parameters
		if rateParam := c.Query("success_rate"); rateParam != "" {
			if rate, err := strconv.ParseFloat(rateParam, 64); err == nil && rate >= 0 && rate <= 100 {
				successRate = rate
			}
		}
	} else {
		// POST/PUT/PATCH - use JSON body
		if err := c.ShouldBindJSON(&req); err == nil && req.SuccessRate >= 0 && req.SuccessRate <= 100 {
			successRate = req.SuccessRate
		}
	}

	// Simulate availability with configurable success rate
	if rand.Float64()*100 < successRate {
		c.JSON(http.StatusOK, gin.H{
			"status":       "available",
			"success_rate": successRate,
			"method":       c.Request.Method,
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":       "unavailable",
			"success_rate": successRate,
			"method":       c.Request.Method,
		})
	}
}

// @Summary      Measure performance
// @Description  Simulate latency with configurable max delay via query parameter
// @Tags         kpi
// @Produce      json
// @Param        max_delay     query     int                     false  "Max delay in ms"        default(500)
// @Success      200           {object}  map[string]interface{}  "latency response"
// @Router       /kpi/performance [get]

// @Summary      Measure performance
// @Description  Simulate latency with configurable max delay
// @Tags         kpi
// @Accept       json
// @Produce      json
// @Param        body          body      PerformanceRequest      false  "Override max_delay via JSON"
// @Success      200           {object}  map[string]interface{} "latency response"
// @Router       /kpi/performance [post]
// @Router       /kpi/performance [put]
// @Router       /kpi/performance [patch]
func performanceHandler(c *gin.Context) {
	var req PerformanceRequest
	maxDelay := 500

	// Handle input based on method
	if c.Request.Method == http.MethodGet {
		// GET method - use query parameters
		if delayParam := c.Query("max_delay"); delayParam != "" {
			if delay, err := strconv.Atoi(delayParam); err == nil && delay > 0 {
				maxDelay = delay
			}
		}
	} else {
		// POST/PUT/PATCH - use JSON body
		if err := c.ShouldBindJSON(&req); err == nil && req.MaxDelay > 0 {
			maxDelay = req.MaxDelay
		}
	}

	// Simulate latency with configurable max delay
	delay := time.Duration(rand.Intn(maxDelay)) * time.Millisecond
	time.Sleep(delay)
	c.JSON(http.StatusOK, gin.H{
		"latency_ms": delay.Milliseconds(),
		"max_delay":  maxDelay,
		"method":     c.Request.Method,
	})
}
// @Summary      Simulate errors
// @Description  Simulate error rate with configurable percentage via query parameter
// @Tags         kpi
// @Produce      json
// @Param        error_rate    query     number                  false  "Error rate (0–100)"     default(5.0)
// @Success      200           {object}  map[string]interface{}  "successful response"
// @Failure      500           {object}  map[string]interface{}  "simulated error response"
// @Router       /kpi/errors [get]

// @Summary      Simulate errors
// @Description  Simulate error rate with configurable percentage
// @Tags         kpi
// @Accept       json
// @Produce      json
// @Param        body          body      ErrorRateRequest        false  "Override error_rate via JSON"
// @Success      200           {object}  map[string]interface{} "successful response"
// @Failure      500           {object}  map[string]interface{} "simulated error response"
// @Router       /kpi/errors [post]
// @Router       /kpi/errors [put]
// @Router       /kpi/errors [patch]
func errorRateHandler(c *gin.Context) {
	var req ErrorRateRequest
	errorRate := 5.0

	// Handle input based on method
	if c.Request.Method == http.MethodGet {
		// GET method - use query parameters
		if rateParam := c.Query("error_rate"); rateParam != "" {
			if rate, err := strconv.ParseFloat(rateParam, 64); err == nil && rate >= 0 && rate <= 100 {
				errorRate = rate
			}
		}
	} else {
		// POST/PUT/PATCH - use JSON body
		if err := c.ShouldBindJSON(&req); err == nil && req.ErrorRate >= 0 && req.ErrorRate <= 100 {
			errorRate = req.ErrorRate
		}
	}

	// Simulate errors with configurable error rate
	if rand.Float64()*100 < errorRate {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "simulated_error",
			"error_rate": errorRate,
			"method":     c.Request.Method,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status":     "success",
			"error_rate": errorRate,
			"method":     c.Request.Method,
		})
	}
}
// @Summary      Run benchmark
// @Description  Simulate a benchmark with configurable delay or max_delay via query parameters
// @Tags         benchmark
// @Produce      json
// @Param        delay         query     int                     false  "Fixed delay in ms"
// @Param        max_delay     query     int                     false  "Max random delay in ms"  default(500)
// @Success      200           {object}  map[string]interface{}  "benchmark latency response"
// @Router       /benchmark [get]

// @Summary      Run benchmark
// @Description  Simulate a benchmark with configurable delay or max_delay
// @Tags         benchmark
// @Accept       json
// @Produce      json
// @Param        body          body      BenchmarkRequest        false  "Override delay/max_delay via JSON"
// @Success      200           {object}  map[string]interface{} "benchmark latency response"
// @Router       /benchmark [post]
// @Router       /benchmark [put]
// @Router       /benchmark [patch]
func benchmarkHandler(c *gin.Context) {
	var req BenchmarkRequest
	var delay time.Duration

	// Handle input based on method
	if c.Request.Method == http.MethodGet {
		// GET method - use query parameters
		if delayParam := c.Query("delay"); delayParam != "" {
			if d, err := strconv.Atoi(delayParam); err == nil && d > 0 {
				delay = time.Duration(d) * time.Millisecond
			}
		}
		
		if delay == 0 {
			maxDelay := 500
			if maxParam := c.Query("max_delay"); maxParam != "" {
				if m, err := strconv.Atoi(maxParam); err == nil && m > 0 {
					maxDelay = m
				}
			}
			delay = time.Duration(rand.Intn(maxDelay)) * time.Millisecond
		}
	} else {
		// POST/PUT/PATCH - use JSON body
		if err := c.ShouldBindJSON(&req); err == nil {
			if req.Delay != nil && *req.Delay > 0 {
				delay = time.Duration(*req.Delay) * time.Millisecond
			} else if req.MaxDelay != nil && *req.MaxDelay > 0 {
				delay = time.Duration(rand.Intn(*req.MaxDelay)) * time.Millisecond
			} else {
				// Default if no valid parameters
				delay = time.Duration(rand.Intn(500)) * time.Millisecond
			}
		} else {
			// Fallback to default if binding fails
			delay = time.Duration(rand.Intn(500)) * time.Millisecond
		}
	}

	start := time.Now()
	time.Sleep(delay)
	latency := time.Since(start)
	
	// Track latency in Datadog
	if span, ok := tracer.SpanFromContext(c.Request.Context()); ok {
		span.SetTag("latency_ms", latency.Milliseconds())
	}
	
	c.JSON(http.StatusOK, gin.H{
		"latency_ms": latency.Milliseconds(),
		"delay_ms":   delay.Milliseconds(),
		"method":     c.Request.Method,
	})
}