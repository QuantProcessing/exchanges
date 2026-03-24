# Backpack Adapter Gap

## Keep

- Market-cache-driven metadata loading and focused request/mapping coverage.
- Localized handling of Backpack-specific client-id constraints.
- Clear exchange-specific mapping seams where the adapter already translates unified behavior into Backpack wire formats.

## Change

- Classify Backpack explicitly as a REST-only adapter for order transport in this first pass.
  Status: landed in the initial rollout.
  Acceptance: constructors set REST mode explicitly, code comments describe the REST-only contract, and deterministic tests assert the REST-only default.
- Converge Backpack SDK naming toward repository-standard request verbs using staged compatibility shims where needed.
  Status: landed in the initial rollout.
  Acceptance: adapter call sites use `GetOrderBook` and `PlaceOrder` as the preferred names for shared SDK query/order concepts, establishing Backpack as the landed repository precedent for that naming family while keeping legacy aliases available for compatibility.
- Clean up stable free-form failures so missing-resource and unsupported paths use shared sentinel errors instead.
  Status: landed in the initial rollout.
  Acceptance: stable unsupported paths return `ErrNotSupported`, stable missing-resource paths use `ErrSymbolNotFound` or the correct shared sentinel, and targeted tests cover those behaviors.
- Add stronger SDK-level tests around constructor seams, private/public REST behavior, and REST-only transport expectations.
  Status: landed in the initial rollout.
  Acceptance: deterministic tests cover the new constructor seams and the compatibility wrapper behavior added in this pass.

## Defer

- Deferred: any future move from REST-only transport to full `OrderMode` switching.
- Deferred: repository-wide decisions about whether stream logic should default back into the main adapter files.
- Deferred: broader repository-wide websocket naming normalization beyond the landed generic `WsClient -> WSClient` convergence pass, including families such as `WebsocketClient`, `BaseWsClient`, and `WsApiClient`.
