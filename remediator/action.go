package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// Outcome is the result of an attempted remediation — recorded as a metric label and
// logged, so every decision the loop makes is auditable.
type Outcome string

const (
	OutcomeHealed          Outcome = "healed"           // we turned the flag off
	OutcomeAlreadySafe     Outcome = "already_safe"     // nothing to do (idempotent)
	OutcomeDryRun          Outcome = "dry_run"          // would have acted, but dry-run
	OutcomeCooldownSkipped Outcome = "cooldown_skipped" // acted too recently for this incident
	OutcomeUnsupported     Outcome = "unsupported"      // no allowlisted action for this alert
	OutcomeFailed          Outcome = "failed"           // action attempted but failed
	OutcomeNeedsHuman      Outcome = "needs_human"      // automation cannot safely progress
)

const cooldownsAnnotation = "omniobserve.io/remediation-cooldowns"

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
	}
}

// DisableFlag turns the named flagd flag off, idempotently and within a per-incident
// cooldown. It returns the outcome (for metrics/logs) and any error. The flagd config is
// a JSON document in a ConfigMap key; flagd hot-reloads the mounted file, so updating the
// ConfigMap is enough to stop the fault — no pod restart.
func (r *FlagRemediator) DisableFlag(ctx context.Context, flag, incidentKey string) (Outcome, error) {
	var outcome Outcome

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Get(ctx, r.configMap, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get configmap %s/%s: %w", r.namespace, r.configMap, err)
		}

		cooldowns, err := readCooldowns(cm.Annotations)
		if err != nil {
			return err
		}
		if inCooldown(cooldowns, incidentKey, r.cooldown) {
			outcome = OutcomeCooldownSkipped
			return nil
		}

		raw, ok := cm.Data[r.configKey]
		if !ok {
			return fmt.Errorf("key %q not in configmap %s/%s", r.configKey, r.namespace, r.configMap)
		}

		var doc map[string]any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			return fmt.Errorf("parse flagd config: %w", err)
		}

		flags, _ := doc["flags"].(map[string]any)
		entry, ok := flags[flag].(map[string]any)
		if !ok {
			outcome = OutcomeUnsupported
			return nil
		}

		if entry["defaultVariant"] == "off" {
			outcome = OutcomeAlreadySafe
			return nil // idempotent: already remediated
		}

		if r.dryRun {
			outcome = OutcomeDryRun
			return nil // dry-run must not mutate the flag or the cooldown annotation
		}

		entry["defaultVariant"] = "off"
		patched, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal flagd config: %w", err)
		}
		cm.Data[r.configKey] = string(patched)
		writeCooldown(cooldowns, incidentKey, time.Now())
		cm.Annotations = writeCooldowns(cm.Annotations, cooldowns)

		if _, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update configmap %s/%s: %w", r.namespace, r.configMap, err)
		}

		outcome = OutcomeHealed
		return nil
	})
	return outcome, err
}

func readCooldowns(annotations map[string]string) (map[string]string, error) {
	raw := annotations[cooldownsAnnotation]
	if raw == "" {
		return map[string]string{}, nil
	}
	var cooldowns map[string]string
	if err := json.Unmarshal([]byte(raw), &cooldowns); err != nil {
		return nil, fmt.Errorf("parse cooldown annotation %q: %w", cooldownsAnnotation, err)
	}
	return cooldowns, nil
}

func writeCooldowns(annotations map[string]string, cooldowns map[string]string) map[string]string {
	if annotations == nil {
		annotations = map[string]string{}
	}
	raw, _ := json.Marshal(cooldowns)
	annotations[cooldownsAnnotation] = string(raw)
	return annotations
}

func inCooldown(cooldowns map[string]string, incidentKey string, cooldown time.Duration) bool {
	raw := cooldowns[incidentKey]
	if raw == "" || cooldown <= 0 {
		return false
	}
	last, err := time.Parse(time.RFC3339Nano, raw)
	return err == nil && time.Since(last) < cooldown
}

func writeCooldown(cooldowns map[string]string, incidentKey string, at time.Time) {
	cooldowns[incidentKey] = at.UTC().Format(time.RFC3339Nano)
}
