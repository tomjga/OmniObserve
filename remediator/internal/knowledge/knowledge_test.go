package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMD(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAndLookup(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "product-catalog.md", `---
service: product-catalog
title: GetProduct fault path
files:
  - src/product-catalog/main.go (GetProduct)
flags: [productCatalogFailure]
---
GetProduct returns codes.Internal when the flag is on.`)
	writeMD(t, dir, "README.md", "ignored")
	writeMD(t, dir, "TEMPLATE.md", "---\nservice: x\n---\nignored")
	writeMD(t, dir, "broken.md", "no frontmatter here")

	entries, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry (README/TEMPLATE/broken skipped), got %d", len(entries))
	}

	// Lookup is normalized: exact label matches.
	e, ok := Lookup(entries, "product-catalog")
	if !ok {
		t.Fatal("expected to find product-catalog")
	}
	if len(e.Files) != 1 || !strings.Contains(e.Files[0], "GetProduct") {
		t.Errorf("files not parsed: %v", e.Files)
	}
	if !strings.Contains(e.Body, "codes.Internal") {
		t.Errorf("body not parsed: %q", e.Body)
	}

	// Render includes the title, the source file, and the body.
	r := e.Render()
	for _, want := range []string{"GetProduct fault path", "src/product-catalog/main.go", "productCatalogFailure", "codes.Internal"} {
		if !strings.Contains(r, want) {
			t.Errorf("render missing %q:\n%s", want, r)
		}
	}

	// Unknown service → no entry (copilot falls back to topology-only).
	if _, ok := Lookup(entries, "shipping"); ok {
		t.Error("did not expect knowledge for shipping")
	}
	if _, ok := Lookup(entries, ""); ok {
		t.Error("empty service must not match")
	}
}

func TestLoad_MissingDir(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected an error for a missing dir so the caller can degrade gracefully")
	}
}
