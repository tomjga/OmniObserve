package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	r.Use(corsMiddleware())
	registerRoutes(r)
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

func TestWebhookBearerAuth(t *testing.T) {
	t.Setenv("WEBHOOK_BEARER_TOKEN", "secret")
	payload := `{"status":"firing","alerts":[]}`

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	newRouter().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	newRouter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want 200", w.Code)
	}
}

func TestApprovalBearerAuth(t *testing.T) {
	t.Setenv("APPROVAL_BEARER_TOKEN", "secret")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/approvals", nil)
	newRouter().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/approvals", nil)
	req.Header.Set("Authorization", "Bearer secret")
	newRouter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want 200", w.Code)
	}
}

func TestListApprovalsRejectsInvalidStatus(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/approvals?status=maybe", nil)
	newRouter().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestApprovalStoreDedupeAndNewestFirst(t *testing.T) {
	store := NewApprovalStore()
	alertA := Alert{
		Status:      "firing",
		Labels:      map[string]string{"alertname": "A", "service": "checkout"},
		Annotations: map[string]string{"summary": "first"},
	}
	alertB := Alert{
		Status:      "firing",
		Labels:      map[string]string{"alertname": "B", "service": "cart"},
		Annotations: map[string]string{"summary": "second"},
	}

	first, created := store.Create(alertA, "flagA")
	if !created {
		t.Fatal("first approval should be created")
	}
	duplicate, created := store.Create(alertA, "flagA")
	if created {
		t.Fatal("duplicate pending approval should not be created")
	}
	if duplicate.ID != first.ID {
		t.Fatalf("duplicate ID = %q, want %q", duplicate.ID, first.ID)
	}
	time.Sleep(time.Nanosecond)
	second, created := store.Create(alertB, "flagB")
	if !created {
		t.Fatal("second approval should be created")
	}

	items := store.List(ApprovalPending)
	if len(items) != 2 {
		t.Fatalf("pending approvals = %d, want 2", len(items))
	}
	if items[0].ID != second.ID || items[1].ID != first.ID {
		t.Fatalf("approvals not sorted newest-first: got %q then %q", items[0].ID, items[1].ID)
	}
}

func TestWebhookUsesCatalogWithoutRemediationAnnotation(t *testing.T) {
	r, cs := newFakeRemediator(t, "on", false, time.Minute)
	oldRemediator, oldCatalog, oldStop := flagRemediator, faultCatalog, autonomousStop
	flagRemediator, faultCatalog, autonomousStop = r, defaultFaultCatalog(), false
	defer func() { flagRemediator, faultCatalog, autonomousStop = oldRemediator, oldCatalog, oldStop }()

	payload := `{"status":"firing","alerts":[
		{"status":"firing","labels":{"alertname":"ProductCatalogHighErrorRate","service":"product-catalog","severity":"critical"},
		 "annotations":{"summary":"catalog should choose productCatalogFailure"}}]}`

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	newRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if v := currentVariant(t, cs); v != "off" {
		t.Fatalf("catalog remediation left flag %q, want off", v)
	}
}

func TestGlobalStopSwitchBlocksMutation(t *testing.T) {
	r, cs := newFakeRemediator(t, "on", false, time.Minute)
	oldRemediator, oldCatalog, oldStop := flagRemediator, faultCatalog, autonomousStop
	flagRemediator, faultCatalog, autonomousStop = r, defaultFaultCatalog(), true
	defer func() { flagRemediator, faultCatalog, autonomousStop = oldRemediator, oldCatalog, oldStop }()

	payload := `{"status":"firing","alerts":[
		{"status":"firing","labels":{"alertname":"ProductCatalogHighErrorRate","service":"product-catalog","severity":"critical"},
		 "annotations":{"summary":"stop switch should block mutation"}}]}`

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	newRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if v := currentVariant(t, cs); v != "on" {
		t.Fatalf("global stop switch mutated flag to %q, want on", v)
	}
}

func TestAutonomyObserveBlocksMutation(t *testing.T) {
	r, cs := newFakeRemediator(t, "on", false, time.Minute)
	oldRemediator, oldCatalog, oldStop, oldMode := flagRemediator, faultCatalog, autonomousStop, autonomyMode
	flagRemediator, faultCatalog, autonomousStop, autonomyMode = r, defaultFaultCatalog(), false, AutonomyObserve
	defer func() {
		flagRemediator, faultCatalog, autonomousStop, autonomyMode = oldRemediator, oldCatalog, oldStop, oldMode
	}()

	payload := `{"status":"firing","alerts":[
		{"status":"firing","labels":{"alertname":"ProductCatalogHighErrorRate","service":"product-catalog","severity":"critical"},
		 "annotations":{"summary":"observe mode should block mutation"}}]}`

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	newRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if v := currentVariant(t, cs); v != "on" {
		t.Fatalf("observe mode mutated flag to %q, want on", v)
	}
}

func TestAutonomyApprovalQueuesAndApprovesMutation(t *testing.T) {
	r, cs := newFakeRemediator(t, "on", false, time.Minute)
	oldRemediator, oldCatalog, oldStop, oldMode := flagRemediator, faultCatalog, autonomousStop, autonomyMode
	oldStore, oldGuard := approvalStore, noopGuard
	flagRemediator, faultCatalog, autonomousStop, autonomyMode = r, defaultFaultCatalog(), false, AutonomyApproval
	approvalStore, noopGuard = NewApprovalStore(), NewNoopStormGuard(3, time.Minute)
	defer func() {
		flagRemediator, faultCatalog, autonomousStop, autonomyMode = oldRemediator, oldCatalog, oldStop, oldMode
		approvalStore, noopGuard = oldStore, oldGuard
	}()

	payload := `{"status":"firing","alerts":[
		{"status":"firing","labels":{"alertname":"ProductCatalogHighErrorRate","service":"product-catalog","severity":"critical"},
		 "annotations":{"summary":"approval should gate mutation"}}]}`

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	newRouter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("webhook status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if v := currentVariant(t, cs); v != "on" {
		t.Fatalf("approval mode mutated flag to %q before approval, want on", v)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/approvals?status=pending", nil)
	newRouter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list approvals status = %d, want 200", w.Code)
	}
	var listed struct {
		Approvals []ApprovalView `json:"approvals"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatalf("approvals body not JSON: %v", err)
	}
	if len(listed.Approvals) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(listed.Approvals))
	}
	if !listed.Approvals[0].CanApprove {
		t.Fatal("pending approval should be approvable")
	}

	w = httptest.NewRecorder()
	body := bytes.NewBufferString(`{"actor":"test","note":"approve fixture remediation"}`)
	req = httptest.NewRequest(http.MethodPost, "/approvals/"+listed.Approvals[0].ID+"/approve", body)
	req.Header.Set("Content-Type", "application/json")
	newRouter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if v := currentVariant(t, cs); v != "off" {
		t.Fatalf("approved remediation left flag %q, want off", v)
	}
}

func TestEffectiveAutonomyUsesSaferMode(t *testing.T) {
	policy := defaultFaultCatalog().Faults[0]
	policy.Safety.Autonomy = string(AutonomyAutoWithVerify)
	if got := effectiveAutonomy(AutonomySuggest, policy); got != AutonomySuggest {
		t.Fatalf("effectiveAutonomy = %q, want suggest", got)
	}
	policy.Safety.Autonomy = string(AutonomyApproval)
	if got := effectiveAutonomy(AutonomyAutoWithVerify, policy); got != AutonomyApproval {
		t.Fatalf("effectiveAutonomy = %q, want approval", got)
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

func TestNoopStormGuard(t *testing.T) {
	g := NewNoopStormGuard(2, time.Minute)
	if g.Record("flag", "incident") {
		t.Fatal("first already-safe outcome should not escalate")
	}
	if g.Record("flag", "incident") {
		t.Fatal("second already-safe outcome at threshold should not escalate")
	}
	if !g.Record("flag", "incident") {
		t.Fatal("third already-safe outcome should escalate")
	}
}
