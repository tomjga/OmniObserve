package main

import "time"

// AlertmanagerWebhook is the JSON payload Alertmanager POSTs to a webhook receiver.
// We model only the fields the remediator actually uses; Alertmanager sends more.
// See https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
type AlertmanagerWebhook struct {
	Version  string  `json:"version"`
	GroupKey string  `json:"groupKey"`
	Status   string  `json:"status"` // "firing" | "resolved"
	Receiver string  `json:"receiver"`
	Alerts   []Alert `json:"alerts"`
}

// Alert is a single alert within the webhook. Labels carry the identity
// (alertname, severity, and any series labels like the offending service);
// annotations carry human text (summary, description) and, by our convention,
// hints the remediator acts on (e.g. a feature-flag name to disable).
type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// alertName returns the alertname label (the human-stable identity of the rule),
// falling back to the fingerprint when unlabeled so metrics never get an empty key.
func (a Alert) alertName() string {
	if n := a.Labels["alertname"]; n != "" {
		return n
	}
	return a.Fingerprint
}

// incidentKey identifies the specific firing instance for idempotency/cooldown:
// the alert rule plus the series it fired on. Two alerts with the same key are the
// "same incident" and must not trigger repeated actions.
func (a Alert) incidentKey() string {
	svc := a.Labels["service"]
	if svc == "" {
		svc = a.Labels["job"]
	}
	return a.alertName() + "|" + svc
}

// remediationFlag is the flagd flag this alert asks the remediator to disable, carried
// as an annotation. Empty means "no flag action for this alert" — the remediator only
// acts when an alert explicitly opts in by naming the flag, never by guessing.
func (a Alert) remediationFlag() string {
	return a.Annotations["remediation_flag"]
}
