# Codebase knowledge (RCA grounding)

Curated, file-backed knowledge about each monitored service's **fault path** — the specific
source file/function that produces the error, how the service is wired to its callers, and
the concrete **code-level fix**. The RCA copilot (`remediator/internal/rca`) looks up the
entry for the service an alert fired on and includes it in the prompt, so its
`## Proposed remediation` / `## Code-level fix` is grounded in the actual code rather than a
generic guess.

It is baked into the remediator image (`COPY knowledge /app/knowledge`, `KNOWLEDGE_DIR`) the
same way the incident corpus (`incidents/`) is — small, dependency-free, and auditable.

## Format

One `*.md` per service with YAML frontmatter:

```yaml
---
service: product-catalog   # must match the alert/series `service` label
title: Short title
files:                     # relevant source paths (key function in parens)
  - src/product-catalog/main.go (GetProduct)
flags: [productCatalogFailure]
---
Markdown body: what the service does, the code path that produces the fault, the blast
radius (how it's connected), and the correct remediation (the bounded action here vs. the
real code-level fix if it were a genuine defect).
```

`TEMPLATE.md` and `README.md` are ignored by the loader. This is the in-context / curated
step of the RAG-over-the-codebase direction — a real retrieval or MCP layer can replace the
loader later without touching the copilot.
