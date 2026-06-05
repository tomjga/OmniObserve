package corpus

import (
	"os"
	"path/filepath"
	"testing"
)

const incA = `---
id: INC-2026-0001
title: Healthy canary auto-aborted by SLO gate
tags: [slo, argo-rollouts, prometheus, false-positive]
services: [api-service]
---
The no-data case evaluated to NaN.
`

const incB = `---
id: INC-2026-0007
title: Feature-flag remediation didn't reach services
tags: [flagd, feature-flags, configmap, propagation]
services: [flagd, product-catalog]
---
flagd served a seed-once copy.
`

func writeCorpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"a.md":        incA,
		"b.md":        incB,
		"TEMPLATE.md": "---\nid: INC-YYYY\n---\nshould be skipped",
		"notes.txt":   "ignored",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoad(t *testing.T) {
	got, err := Load(writeCorpus(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("loaded %d incidents, want 2 (TEMPLATE/.txt skipped)", len(got))
	}
}

func TestRetrieve_RanksByOverlap(t *testing.T) {
	incidents, _ := Load(writeCorpus(t))

	// An alert about product-catalog + flagd should surface INC-0007 first.
	got := Retrieve(incidents, []string{"product-catalog", "flagd", "feature-flags"}, 2)
	if len(got) == 0 {
		t.Fatal("expected at least one match")
	}
	if got[0].ID != "INC-2026-0007" {
		t.Errorf("top match = %s, want INC-2026-0007", got[0].ID)
	}
}

func TestRetrieve_DropsZeroOverlap(t *testing.T) {
	incidents, _ := Load(writeCorpus(t))
	got := Retrieve(incidents, []string{"kafka", "oom"}, 5)
	if len(got) != 0 {
		t.Errorf("expected no matches for unrelated terms, got %d", len(got))
	}
}
