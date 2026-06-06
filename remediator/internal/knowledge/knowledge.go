// Package knowledge loads curated codebase knowledge (knowledge/*.md) and looks up the
// entry for the service an incident fired on. It is what lets the RCA copilot propose a
// remediation grounded in the ACTUAL code path — the specific file and function that
// produces the error, how the service is wired to its callers, and the concrete code-level
// fix — instead of a generic suggestion.
//
// This is deliberately the same shape as the incident corpus: small, file-backed, and
// dependency-free, so the grounding is auditable (you can see exactly what the copilot was
// told). It is the in-context / curated step of the RAG-over-the-codebase direction; a real
// retrieval or MCP layer can replace Load/Lookup later without touching the copilot.
package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Entry is curated knowledge about one service's fault path.
type Entry struct {
	Service string   `yaml:"service"` // the alert/series `service` label this applies to
	Title   string   `yaml:"title"`
	Files   []string `yaml:"files"` // relevant source paths (with the key function in parens)
	Flags   []string `yaml:"flags"` // flagd flags that exercise this path
	Body    string   `yaml:"-"`     // markdown after the frontmatter: wiring, mechanism, fix
}

var frontmatter = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)$`)

// Load parses every *.md in dir (skipping TEMPLATE.md and README.md) into Entries. A file
// that doesn't parse is skipped, not fatal — one malformed note must not blind the copilot
// to the rest. A missing dir returns (nil, err) so the caller can degrade to ungrounded.
func Load(dir string) ([]Entry, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, f := range files {
		name := f.Name()
		if f.IsDir() || !strings.HasSuffix(name, ".md") || name == "TEMPLATE.md" || name == "README.md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		m := frontmatter.FindSubmatch(raw)
		if m == nil {
			continue
		}
		var e Entry
		if err := yaml.Unmarshal(m[1], &e); err != nil {
			continue
		}
		e.Body = strings.TrimSpace(string(m[2]))
		if e.Service != "" {
			out = append(out, e)
		}
	}
	return out, nil
}

// Lookup returns the entry for a service, matching on the normalized service label. Returns
// ok=false when there's no curated knowledge — the copilot then falls back to topology-only
// reasoning rather than inventing code it can't see.
func Lookup(entries []Entry, service string) (Entry, bool) {
	want := normalize(service)
	if want == "" {
		return Entry{}, false
	}
	for _, e := range entries {
		if normalize(e.Service) == want {
			return e, true
		}
	}
	return Entry{}, false
}

// Render formats an entry for the prompt: title, the source files it points at, and the
// curated body (mechanism + how-connected + the code-level fix).
func (e Entry) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s (service: %s)\n", e.Title, e.Service)
	if len(e.Files) > 0 {
		fmt.Fprintf(&b, "Relevant source:\n")
		for _, f := range e.Files {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	if len(e.Flags) > 0 {
		fmt.Fprintf(&b, "Fault flags: %s\n", strings.Join(e.Flags, ", "))
	}
	b.WriteString("\n")
	b.WriteString(e.Body)
	return b.String()
}

var nonword = regexp.MustCompile(`[^a-z0-9]+`)

func normalize(s string) string {
	return nonword.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "")
}
