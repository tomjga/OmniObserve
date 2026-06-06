---
service: cart
title: Cart Service — EmptyCart / AddItem fault path
files:
  - src/cart/src/services/CartService.cs (EmptyCartAsync, AddItemAsync)
  - src/cart/src/cartstore/ (ValkeyCartStore — backing store)
flags: [cartFailure]
---
**What the service does.** `cart` is a .NET gRPC service backing the shopping cart
(`AddItem`, `GetCart`, `EmptyCart`) over a Valkey/Redis store. It is called by the frontend
and by `checkout` (which empties the cart after placing an order).

**How the fault is produced (the code path).** The cart RPCs evaluate the `cartFailure`
feature flag via the OpenFeature/flagd client. When `on`, `EmptyCart` (and related writes)
throw `RpcException(new Status(StatusCode.Internal, ...))` instead of completing, so the
call fails server-side.

**Blast radius (how it's connected).** `checkout` calls `EmptyCart` at the end of
`PlaceOrder`, so a cart fault can surface as a checkout failure as well as a frontend
error. Under the demo's default load, cart write paths are exercised less than product
reads, so the signal accumulates more slowly than product-catalog.

**Correct remediation.**
- *In this environment*: disable the `cartFailure` flag (the remediator's bounded action).
- *If this were a real defect*: distinguish a transient store error from a logic error.
  For the backing-store call, add a bounded retry with timeout and surface
  `StatusCode.Unavailable` (retryable) rather than `Internal`; make `EmptyCart` idempotent
  so `checkout` can safely retry. That prevents a single store blip from failing the whole
  order flow.
