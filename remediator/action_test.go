package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// flagdConfig builds a minimal demo.flagd.json with productCatalogFailure at the given
// defaultVariant, so tests can start from "on" (fault active) or "off" (already healed).
func flagdConfig(defaultVariant string) string {
	doc := map[string]any{
		"flags": map[string]any{
			"productCatalogFailure": map[string]any{
				"state":          "ENABLED",
				"variants":       map[string]any{"on": true, "off": false},
				"defaultVariant": defaultVariant,
			},
		},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

func newFakeRemediator(t *testing.T, defaultVariant string, dryRun bool, cooldown time.Duration) (*FlagRemediator, *fake.Clientset) {
	t.Helper()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "flagd-config", Namespace: "otel-demo"},
		Data:       map[string]string{"demo.flagd.json": flagdConfig(defaultVariant)},
	}
	cs := fake.NewClientset(cm)
	return NewFlagRemediator(cs, "otel-demo", "flagd-config", "demo.flagd.json", dryRun, cooldown), cs
}

// currentVariant reads productCatalogFailure's defaultVariant back from the cluster.
func currentVariant(t *testing.T, cs *fake.Clientset) string {
	t.Helper()
	cm, err := cs.CoreV1().ConfigMaps("otel-demo").Get(context.Background(), "flagd-config", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get configmap: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(cm.Data["demo.flagd.json"]), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	flags := doc["flags"].(map[string]any)
	return flags["productCatalogFailure"].(map[string]any)["defaultVariant"].(string)
}

func TestDisableFlag_TurnsOff(t *testing.T) {
	r, cs := newFakeRemediator(t, "on", false, time.Minute)
	got, err := r.DisableFlag(context.Background(), "productCatalogFailure", "inc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != OutcomeDisabled {
		t.Errorf("outcome = %q, want disabled", got)
	}
	if v := currentVariant(t, cs); v != "off" {
		t.Errorf("flag defaultVariant = %q, want off", v)
	}
}

func TestDisableFlag_Idempotent(t *testing.T) {
	r, cs := newFakeRemediator(t, "off", false, time.Minute)
	got, err := r.DisableFlag(context.Background(), "productCatalogFailure", "inc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != OutcomeAlreadyOff {
		t.Errorf("outcome = %q, want already_off", got)
	}
	if v := currentVariant(t, cs); v != "off" {
		t.Errorf("flag defaultVariant = %q, want off", v)
	}
}

func TestDisableFlag_DryRunDoesNotMutate(t *testing.T) {
	r, cs := newFakeRemediator(t, "on", true, time.Minute)
	got, err := r.DisableFlag(context.Background(), "productCatalogFailure", "inc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != OutcomeDryRun {
		t.Errorf("outcome = %q, want dry_run", got)
	}
	if v := currentVariant(t, cs); v != "on" {
		t.Errorf("dry-run mutated the flag to %q, want it left on", v)
	}
}

func TestDisableFlag_Cooldown(t *testing.T) {
	r, _ := newFakeRemediator(t, "on", false, time.Minute)
	if _, err := r.DisableFlag(context.Background(), "productCatalogFailure", "inc1"); err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Same incident again within the window: must not act.
	got, err := r.DisableFlag(context.Background(), "productCatalogFailure", "inc1")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if got != OutcomeCooldown {
		t.Errorf("outcome = %q, want cooldown", got)
	}
}

func TestDisableFlag_Missing(t *testing.T) {
	r, _ := newFakeRemediator(t, "on", false, time.Minute)
	got, err := r.DisableFlag(context.Background(), "noSuchFlag", "inc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != OutcomeFlagMissing {
		t.Errorf("outcome = %q, want flag_missing", got)
	}
}
