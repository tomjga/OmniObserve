// Package corpus loads the incident-RCA corpus (incidents/*.md) and retrieves the most
// relevant past incidents for a new alert. Retrieval is deliberately simple — tag,
// service, and title-keyword overlap, no embeddings or vector DB. At this corpus size
// that's accurate, explainable, and dependency-free, and it makes the RCA copilot's
// grounding auditable: you can see exactly which precedents it was given.
package corpus

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Incident is one RCA from the corpus.
type Incident struct {
	ID       string   `yaml:"id"`
	Title    string   `yaml:"title"`
	Tags     []string `yaml:"tags"`
	Services []string `yaml:"services"`
	Body     string   `yaml:"-"` // markdown after the frontmatter
}

var frontmatter = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)$`)

// Load parses every *.md in dir (skipping TEMPLATE.md and README.md) into Incidents.
// A file that doesn't parse is skipped, not fatal — one malformed RCA must not blind
// the copilot to the rest of the corpus.
func Load(dir string) ([]Incident, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Incident
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "TEMPLATE.md" || name == "README.md" {
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
		var inc Incident
		if err := yaml.Unmarshal(m[1], &inc); err != nil {
			continue
		}
		inc.Body = strings.TrimSpace(string(m[2]))
		if inc.ID != "" {
			out = append(out, inc)
		}
	}
	return out, nil
}

type scored struct {
	inc   Incident
	score int
}

// Retrieve returns up to k incidents most relevant to the query terms, scoring tag and
// service matches highest and title-word overlap lower. Incidents with zero overlap are
// dropped — better to give the LLM nothing than irrelevant precedent.
func Retrieve(incidents []Incident, terms []string, k int) []Incident {
	want := map[string]bool{}
	for _, t := range terms {
		if t = normalize(t); t != "" {
			want[t] = true
		}
	}

	var ranked []scored
	for _, inc := range incidents {
		s := 0
		for _, tag := range inc.Tags {
			if want[normalize(tag)] {
				s += 3
			}
		}
		for _, svc := range inc.Services {
			if want[normalize(svc)] {
				s += 3
			}
		}
		for _, w := range strings.Fields(inc.Title) {
			if want[normalize(w)] {
				s++
			}
		}
		if s > 0 {
			ranked = append(ranked, scored{inc, s})
		}
	}

	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if len(ranked) > k {
		ranked = ranked[:k]
	}
	out := make([]Incident, len(ranked))
	for i, r := range ranked {
		out[i] = r.inc
	}
	return out
}

var nonword = regexp.MustCompile(`[^a-z0-9]+`)

// normalize lowercases and strips punctuation so "Argo-Rollouts" and "argo rollouts"
// match on the "argo"/"rollouts" tokens.
func normalize(s string) string {
	return nonword.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "")
}
