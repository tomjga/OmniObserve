package rca

import (
	"fmt"
	"strings"

	"github.com/tomjga/OmniObserve/remediator/internal/corpus"
	"github.com/tomjga/OmniObserve/remediator/internal/evidence"
)

type Confidence struct {
	Score              float64
	Level              string
	EvidenceCoverage   string
	CorpusMatchQuality string
	RecoverySignal     string
	Rationale          []string
}

func AssessConfidence(metrics []evidence.Metric, precedent []corpus.Incident, verification string) Confidence {
	c := Confidence{
		EvidenceCoverage:   evidenceCoverage(metrics),
		CorpusMatchQuality: corpusMatchQuality(precedent),
		RecoverySignal:     recoverySignal(verification),
	}

	switch c.EvidenceCoverage {
	case "strong":
		c.Score += 0.35
		c.Rationale = append(c.Rationale, "multiple Prometheus evidence points were available")
	case "partial":
		c.Score += 0.22
		c.Rationale = append(c.Rationale, "one Prometheus evidence point was available")
	default:
		c.Score += 0.08
		c.Rationale = append(c.Rationale, "Prometheus evidence was thin or unavailable")
	}

	switch c.CorpusMatchQuality {
	case "strong":
		c.Score += 0.30
		c.Rationale = append(c.Rationale, "multiple related prior incidents were retrieved")
	case "partial":
		c.Score += 0.18
		c.Rationale = append(c.Rationale, "one related prior incident was retrieved")
	default:
		c.Score += 0.05
		c.Rationale = append(c.Rationale, "no close prior incident was retrieved")
	}

	switch c.RecoverySignal {
	case "recovered":
		c.Score += 0.35
		c.Rationale = append(c.Rationale, "post-action signal improved")
	case "not_recovered":
		c.Score += 0.10
		c.Rationale = append(c.Rationale, "post-action signal did not improve")
	case "unknown":
		c.Score += 0.15
		c.Rationale = append(c.Rationale, "post-action recovery signal is unavailable or pending")
	default:
		c.Score += 0.08
		c.Rationale = append(c.Rationale, "post-action verification was not applicable")
	}

	if c.Score >= 0.80 {
		c.Level = "high"
	} else if c.Score >= 0.50 {
		c.Level = "medium"
	} else {
		c.Level = "low"
	}
	return c
}

func (c Confidence) Render() string {
	return fmt.Sprintf("- Score: %.2f (%s)\n- Evidence coverage: %s\n- Corpus match quality: %s\n- Recovery signal: %s\n- Rationale: %s\n",
		c.Score, c.Level, c.EvidenceCoverage, c.CorpusMatchQuality, c.RecoverySignal, strings.Join(c.Rationale, "; "))
}

func evidenceCoverage(metrics []evidence.Metric) string {
	if len(metrics) >= 2 {
		return "strong"
	}
	if len(metrics) == 1 {
		return "partial"
	}
	return "weak"
}

func corpusMatchQuality(precedent []corpus.Incident) string {
	if len(precedent) >= 2 {
		return "strong"
	}
	if len(precedent) == 1 {
		return "partial"
	}
	return "weak"
}

func recoverySignal(verification string) string {
	switch strings.ToLower(strings.TrimSpace(verification)) {
	case "improved":
		return "recovered"
	case "not_improved":
		return "not_recovered"
	case "no_baseline", "no_after_data", "pending", "not_configured", "cancelled", "":
		return "unknown"
	default:
		return "not_applicable"
	}
}
