package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type FaultCatalog struct {
	Faults []FaultPolicy `json:"faults"`
}

type FaultPolicy struct {
	Service string `json:"service"`
	Symptom string `json:"symptom"`
	Alert   string `json:"alert"`
	Action  struct {
		Type string `json:"type"`
		Flag string `json:"flag"`
	} `json:"action"`
	Safety struct {
		Cooldown          string `json:"cooldown"`
		Autonomy          string `json:"autonomy"`
		MaxActionsPerHour int    `json:"maxActionsPerHour"`
	} `json:"safety"`
	Evidence struct {
		PrometheusQueries []string `json:"prometheusQueries"`
	} `json:"evidence"`
}

func LoadFaultCatalog(path string) (*FaultCatalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return defaultFaultCatalog(), err
	}
	var catalog FaultCatalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return defaultFaultCatalog(), fmt.Errorf("parse fault catalog %s: %w", path, err)
	}
	if len(catalog.Faults) == 0 {
		return defaultFaultCatalog(), fmt.Errorf("fault catalog %s has no faults", path)
	}
	return &catalog, nil
}

func defaultFaultCatalog() *FaultCatalog {
	const raw = `{
  "faults": [
    {"service":"product-catalog","symptom":"high-grpc-error-rate","alert":"ProductCatalogHighErrorRate","action":{"type":"disable-flag","flag":"productCatalogFailure"},"safety":{"cooldown":"5m","autonomy":"auto-with-verify","maxActionsPerHour":1},"evidence":{"prometheusQueries":["error_ratio","request_rate"]}},
    {"service":"ad","symptom":"high-grpc-error-rate","alert":"AdHighErrorRate","action":{"type":"disable-flag","flag":"adFailure"},"safety":{"cooldown":"5m","autonomy":"auto-with-verify","maxActionsPerHour":1},"evidence":{"prometheusQueries":["error_ratio","request_rate"]}},
    {"service":"cart","symptom":"high-grpc-error-rate","alert":"CartHighErrorRate","action":{"type":"disable-flag","flag":"cartFailure"},"safety":{"cooldown":"5m","autonomy":"auto-with-verify","maxActionsPerHour":1},"evidence":{"prometheusQueries":["error_ratio","request_rate"]}}
  ]
}`
	var catalog FaultCatalog
	_ = json.Unmarshal([]byte(raw), &catalog)
	return &catalog
}

func (c *FaultCatalog) Lookup(alert Alert) (FaultPolicy, bool) {
	if c == nil {
		return FaultPolicy{}, false
	}
	for _, f := range c.Faults {
		if f.Alert == alert.alertName() && f.Service == alert.serviceName() {
			return f, true
		}
	}
	return FaultPolicy{}, false
}
