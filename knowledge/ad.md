---
service: ad
title: Ad Service — GetAds fault path
files:
  - src/ad/src/main/java/oteldemo/AdService.java (getAds, checkAdFailure)
flags: [adFailure]
---
**What the service does.** `ad` is a Java gRPC service whose `GetAds` RPC returns banner
ads for a set of context keys. It is called by the frontend when rendering pages.

**How the fault is produced (the code path).** `GetAds` evaluates the `adFailure` feature
flag via the OpenFeature/flagd client. When `on`, the handler throws (a
`StatusRuntimeException` with `Status.UNAVAILABLE`/`INTERNAL`) instead of returning ads, so
the gRPC call fails and `rpc_server_duration` records a non-zero `rpc_grpc_status_code`.

**Blast radius (how it's connected).** Only the frontend calls `ad`, and ads are a
non-critical, best-effort page element. The fault is therefore lower-traffic and lower-
severity than product-catalog — under the demo's default load `ad` receives far fewer
requests, so its error ratio moves slowly.

**Correct remediation.**
- *In this environment*: disable the `adFailure` flag (the remediator's bounded action).
- *If this were a real defect*: ads are non-essential, so `GetAds` should fail soft —
  catch the downstream error and return an empty `AdResponse` (or a cached default) rather
  than propagating a gRPC error to the frontend. Degrading ads to "no ad shown" keeps the
  page healthy and avoids spending error budget on a non-critical feature.
