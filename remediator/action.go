package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Outcome is the result of an attempted remediation — recorded as a metric label and
// logged, so every decision the loop makes is auditable.
type Outcome string

const (
	OutcomeDisabled    Outcome = "disabled"     // we turned the flag off
	OutcomeAlreadyOff  Outcome = "already_off"  // nothing to do (idempotent)
	OutcomeDryRun      Outcome = "dry_run"      // would have acted, but dry-run
	OutcomeCooldown    Outcome = "cooldown"     // acted too recently for this incident
	OutcomeFlagMissing Outcome = "flag_missing" // alert named a flag flagd doesn't have
)

// FlagRemediator disables flagd feature flags in response to alerts. It is the bounded
// action of OmniObserve's control loop: the ONLY mutation it can perform is setting a
// named flag's defaultVariant to "off" — a feature-flag kill switch, the safest possible
// remediation (reversible, scoped, and exactly undoing the injected fault).
type FlagRemediator struct {
	k8s       kubernetes.Interface
	namespace string // where the flagd ConfigMap lives (e.g. otel-demo)
	configMap string // e.g. flagd-config
	configKey string // e.g. demo.flagd.json
	dryRun    bool
	cooldown  time.Duration

	mu        sync.Mutex
	lastActed map[string]time.Time // incidentKey -> last action time (cooldown)
}

// NewFlagRemediator builds a remediator bound to a flagd ConfigMap.
func NewFlagRemediator(k8s kubernetes.Interface, namespace, configMap, configKey string, dryRun bool, cooldown time.Duration) *FlagRemediator {
	return &FlagRemediator{
		k8s:       k8s,
		namespace: namespace,
		configMap: configMap,
		configKey: configKey,
		dryRun:    dryRun,
		cooldown:  cooldown,
		lastActed: map[string]time.Time{},
	}
}

// DisableFlag turns the named flagd flag off, idempotently and within a per-incident
// cooldown. It returns the outcome (for metrics/logs) and any error. The flagd config is
// a JSON document in a ConfigMap key; flagd hot-reloads the mounted file, so updating the
// ConfigMap is enough to stop the fault — no pod restart.
func (r *FlagRemediator) DisableFlag(ctx context.Context, flag, incidentKey string) (Outcome, error) {
	// Cooldown: never act twice on the same incident within the window. This is what
	// keeps the loop from thrashing when an alert keeps firing while it recovers.
	r.mu.Lock()
	if last, ok := r.lastActed[incidentKey]; ok && time.Since(last) < r.cooldown {
		r.mu.Unlock()
		return OutcomeCooldown, nil
	}
	r.mu.Unlock()

	cm, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Get(ctx, r.configMap, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get configmap %s/%s: %w", r.namespace, r.configMap, err)
	}

	raw, ok := cm.Data[r.configKey]
	if !ok {
		return "", fmt.Errorf("key %q not in configmap %s/%s", r.configKey, r.namespace, r.configMap)
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return "", fmt.Errorf("parse flagd config: %w", err)
	}

	flags, _ := doc["flags"].(map[string]any)
	entry, ok := flags[flag].(map[string]any)
	if !ok {
		return OutcomeFlagMissing, nil
	}

	if entry["defaultVariant"] == "off" {
		return OutcomeAlreadyOff, nil // idempotent: already remediated
	}

	if r.dryRun {
		// Mark acted so we don't re-log the same intent every evaluation, but never mutate.
		r.markActed(incidentKey)
		return OutcomeDryRun, nil
	}

	entry["defaultVariant"] = "off"
	patched, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal flagd config: %w", err)
	}
	cm.Data[r.configKey] = string(patched)

	if _, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return "", fmt.Errorf("update configmap %s/%s: %w", r.namespace, r.configMap, err)
	}

	r.markActed(incidentKey)
	return OutcomeDisabled, nil
}

func (r *FlagRemediator) markActed(incidentKey string) {
	r.mu.Lock()
	r.lastActed[incidentKey] = time.Now()
	r.mu.Unlock()
}
