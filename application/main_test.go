package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TestMain wires the test environment once. Assigning a no-op logger here mirrors
// the production fix: handlers log via the package-level `logger`, so a nil logger
// would panic (the bug this project fixed in healthHandler).
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	logger = zap.NewNop().Sugar()
	os.Exit(m.Run())
}

// serve runs a single request against one handler in isolation (no OTel/metrics
// middleware) and returns the recorded response.
func serve(handler gin.HandlerFunc, method, target string) *httptest.ResponseRecorder {
	r := gin.New()
	r.Any("/t", handler)
	req := httptest.NewRequest(method, "/t"+target, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v (body=%q)", err, w.Body.String())
	}
	return body
}

// The KPI handlers are randomized, so we assert at the deterministic boundaries
// (rate 0 and 100) and repeat to catch any off-by-one in the comparison.
const iterations = 50

func TestAvailability_FullSuccessRateAlwaysUp(t *testing.T) {
	for i := 0; i < iterations; i++ {
		if w := serve(availabilityHandler, http.MethodGet, "?success_rate=100"); w.Code != http.StatusOK {
			t.Fatalf("success_rate=100: want 200, got %d", w.Code)
		}
	}
}

func TestAvailability_ZeroSuccessRateAlwaysDown(t *testing.T) {
	for i := 0; i < iterations; i++ {
		if w := serve(availabilityHandler, http.MethodGet, "?success_rate=0"); w.Code != http.StatusServiceUnavailable {
			t.Fatalf("success_rate=0: want 503, got %d", w.Code)
		}
	}
}

func TestErrors_FullErrorRateAlwaysFails(t *testing.T) {
	for i := 0; i < iterations; i++ {
		if w := serve(errorRateHandler, http.MethodGet, "?error_rate=100"); w.Code != http.StatusInternalServerError {
			t.Fatalf("error_rate=100: want 500, got %d", w.Code)
		}
	}
}

func TestErrors_ZeroErrorRateAlwaysSucceeds(t *testing.T) {
	for i := 0; i < iterations; i++ {
		if w := serve(errorRateHandler, http.MethodGet, "?error_rate=0"); w.Code != http.StatusOK {
			t.Fatalf("error_rate=0: want 200, got %d", w.Code)
		}
	}
}

func TestPerformance_LatencyWithinMaxDelay(t *testing.T) {
	const maxDelay = 20
	w := serve(performanceHandler, http.MethodGet, "?max_delay=20")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	lat, ok := decode(t, w)["latency_ms"].(float64)
	if !ok {
		t.Fatal("response missing numeric latency_ms")
	}
	if lat < 0 || lat >= maxDelay {
		t.Fatalf("latency_ms=%v outside [0,%d)", lat, maxDelay)
	}
}

func TestBenchmark_ReportsLatency(t *testing.T) {
	w := serve(benchmarkHandler, http.MethodGet, "?delay=5")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if _, ok := decode(t, w)["latency_ms"].(float64); !ok {
		t.Fatal("response missing numeric latency_ms")
	}
}

// TestHealth_NoNilLoggerPanic guards the regression that healthHandler panicked
// on a nil package-level logger.
func TestHealth_NoNilLoggerPanic(t *testing.T) {
	w := serve(healthHandler, http.MethodGet, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if decode(t, w)["status"] != "ok" {
		t.Fatalf("want status=ok, got %v", w.Body.String())
	}
}
