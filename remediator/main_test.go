package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TestMain wires a no-op logger so handlers that log don't panic on a nil logger
// (the exact regression that bit api-service's healthHandler).
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	logger = zap.NewNop().Sugar()
	m.Run()
}

func newRouter() *gin.Engine {
	r := gin.New()
	r.POST("/webhook", webhookHandler)
	r.GET("/healthz", healthHandler)
	return r
}

func TestHealthHandler(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	newRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("healthz body not JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("healthz status field = %q, want ok", body["status"])
	}
	if body["version"] == "" {
		t.Error("healthz did not report a version")
	}
}

func TestWebhookHandler(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		wantCode int
		wantRecv float64
	}{
		{
			name: "firing alert is accepted",
			payload: `{"status":"firing","alerts":[
				{"status":"firing","labels":{"alertname":"HighErrorRate","service":"cart","severity":"critical"},
				 "annotations":{"summary":"cart 5xx burning SLO"}}]}`,
			wantCode: http.StatusOK,
			wantRecv: 1,
		},
		{
			name:     "empty alert list is valid",
			payload:  `{"status":"resolved","alerts":[]}`,
			wantCode: http.StatusOK,
			wantRecv: 0,
		},
		{
			name:     "malformed JSON is rejected",
			payload:  `{not json`,
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			newRouter().ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantCode, w.Body.String())
			}
			if tt.wantCode == http.StatusOK {
				var body map[string]float64
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("body not JSON: %v", err)
				}
				if body["received"] != tt.wantRecv {
					t.Errorf("received = %v, want %v", body["received"], tt.wantRecv)
				}
			}
		})
	}
}

// TestAlertKeys covers the label fallbacks so metrics/idempotency keys are never empty.
func TestAlertKeys(t *testing.T) {
	withName := Alert{Labels: map[string]string{"alertname": "HighErrorRate", "service": "cart"}}
	if got := withName.alertName(); got != "HighErrorRate" {
		t.Errorf("alertName = %q, want HighErrorRate", got)
	}
	if got := withName.incidentKey(); got != "HighErrorRate|cart" {
		t.Errorf("incidentKey = %q, want HighErrorRate|cart", got)
	}

	// No alertname -> fall back to fingerprint; service falls back to job.
	noName := Alert{Fingerprint: "abc123", Labels: map[string]string{"job": "api-service"}}
	if got := noName.alertName(); got != "abc123" {
		t.Errorf("alertName fallback = %q, want abc123", got)
	}
	if got := noName.incidentKey(); got != "abc123|api-service" {
		t.Errorf("incidentKey fallback = %q, want abc123|api-service", got)
	}
}
