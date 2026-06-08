package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	env := map[string]string{
		"WORKER_TARGET_URL":  "http://example.test",
		"WORKER_CONCURRENCY": "4",
		"WORKER_INTERVAL_MS": "250",
		"WORKER_LISTEN_ADDR": ":9090",
	}
	cfg := configFromEnv(func(k string) string { return env[k] })
	if cfg.TargetURL != "http://example.test" {
		t.Fatalf("TargetURL = %q", cfg.TargetURL)
	}
	if cfg.Workers != 4 {
		t.Fatalf("Workers = %d", cfg.Workers)
	}
	if cfg.Interval != 250*time.Millisecond {
		t.Fatalf("Interval = %s", cfg.Interval)
	}
	if cfg.ListenAddr != ":9090" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
}

func TestDoRequestHitsTarget(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	doRequest(t.Context(), srv.Client(), srv.URL, 1)
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
}
