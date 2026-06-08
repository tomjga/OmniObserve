package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFaultCatalogLookup(t *testing.T) {
	catalog := defaultFaultCatalog()
	policy, ok := catalog.Lookup(Alert{
		Labels: map[string]string{"alertname": "CartHighErrorRate", "service": "cart"},
	})
	if !ok {
		t.Fatal("expected cart policy")
	}
	if policy.Action.Flag != "cartFailure" {
		t.Fatalf("flag = %q, want cartFailure", policy.Action.Flag)
	}
}

func TestLoadFaultCatalogFallback(t *testing.T) {
	catalog, err := LoadFaultCatalog(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected missing file error")
	}
	if len(catalog.Faults) == 0 {
		t.Fatal("fallback catalog is empty")
	}
}

func TestLoadFaultCatalogFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	raw := `{"faults":[{"service":"svc","alert":"SvcHighErrorRate","action":{"type":"disable-flag","flag":"svcFailure"}}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := LoadFaultCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	policy, ok := catalog.Lookup(Alert{Labels: map[string]string{"alertname": "SvcHighErrorRate", "service": "svc"}})
	if !ok || policy.Action.Flag != "svcFailure" {
		t.Fatalf("lookup = (%+v, %v), want svcFailure", policy, ok)
	}
}
