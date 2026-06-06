---
service: product-catalog
title: Product Catalog — GetProduct fault path
files:
  - src/product-catalog/main.go (GetProduct, checkProductFailure)
  - src/product-catalog/products/ (product data loading)
flags: [productCatalogFailure]
---
**What the service does.** `product-catalog` is a Go gRPC service that serves product data
(`ListProducts`, `GetProduct`, `SearchProducts`). It is one of the most-called services: the
frontend renders every product page through it, and `recommendation` and `checkout` call it
server-to-server.

**How the fault is produced (the code path).** `GetProduct` calls a helper that reads the
`productCatalogFailure` feature flag from flagd via the OpenFeature client. When the flag's
variant is `on`, the handler short-circuits before returning data and responds with
`status.Errorf(codes.Internal, "ProductCatalogService Fail Feature Flag Enabled")`. That
gRPC `INTERNAL` (code 13) is what the OTel instrumentation records on
`rpc_server_duration_milliseconds_count{rpc_grpc_status_code="13"}`, which is exactly the
numerator of the `ProductCatalogHighErrorRate` SLO query.

**Blast radius (how it's connected).** Because GetProduct sits on the hot path, the error
fans out: the frontend returns 5xx for product pages, and `recommendation`/`checkout`
calls that depend on product data fail too. This is why a single flag flip moves the
service's error ratio sharply and quickly.

**Correct remediation.**
- *In this environment* the trigger is the injected `productCatalogFailure` flag, not a code
  defect. The right, sufficient fix is to set the flag's `defaultVariant` back to `off` —
  which is the bounded, reversible action the remediator already performs (flagd hot-reloads
  the ConfigMap, so consumers recover without a restart).
- *If this were a genuine runtime defect* (the same INTERNAL surfacing from real data/
  dependency errors rather than the flag), the code-level fix is in `GetProduct`: stop
  collapsing every failure into `codes.Internal`. Return `codes.NotFound` for an unknown
  product id, validate the request id up front, and wrap the data-loading error with
  context (`fmt.Errorf("load product %q: %w", id, err)`) so callers and traces can tell a
  missing product from a backend outage. That keeps the SLO query honest (a 404 is not an
  error budget burn) and gives `recommendation`/`checkout` a typed error to handle instead
  of a blanket 5xx.
