# Bitget WebSocket Order Transport Design

## Goal

Extend the existing `bitget` adapter so order operations can use WebSocket transport in both supported account modes:

- UTA
- Classic

This change must preserve the already-verified REST path and only add WS order transport when the adapter is explicitly switched to `OrderModeWS`.

## User-Confirmed Scope

The user explicitly wants the more complete path:

- support WS trading for both UTA and Classic
- use Binance perp only as a reference for transport structure, not as a protocol template
- keep the design low-disruption to the current Bitget adapter

This design does not attempt to:

- change the default order mode away from REST
- add batch WS trading
- redesign public market-data code

## Current State

The current `bitget` package already has:

- `privateProfile` split by account mode
- auto-detection between UTA and Classic for private capabilities
- one private WS client per adapter for subscriptions
- REST implementations for place/cancel/modify order in both profiles

What it does not have yet:

- request/response correlation for private WS trade requests
- WS trade request encoders for UTA
- WS trade request encoders for Classic spot and Classic contract
- adapter-level routing from `OrderModeWS` to those paths

## Why A Profile-Level WS Design Is Required

The low-change path is not “make Bitget WS look like Binance”. The low-change path is:

1. Keep public code unchanged.
2. Keep adapter method signatures unchanged.
3. Keep account-mode branching inside the existing private profiles.
4. Add a reusable WS trade request layer in `bitget/sdk/private_ws.go`.

This is required because the official Bitget protocols differ materially:

- UTA uses `op=trade`, a top-level `id`, top-level `topic`, and top-level `category`, with `args` carrying trade payloads.
- Classic spot uses `op=trade`, but the per-request metadata lives inside `args[0]` as `id`, `instType`, `instId`, and `channel`.
- Classic contract follows the same broad style as classic spot, but the required fields differ and include futures-specific trade parameters.

So the right abstraction boundary is:

- shared transport machinery in SDK
- protocol-specific message builders per account mode
- profile-level routing in the adapter layer

## External API Facts This Design Depends On

The design depends on the following current official Bitget behavior:

- UTA private WS place-order uses:
  - `op: "trade"`
  - top-level `id`
  - top-level `topic: "place-order"`
  - top-level `category`
  - `args` list with one order payload
- UTA private WS modify-order uses the same top-level shape with `topic: "modify-order"`
- UTA modify acknowledgements are asynchronous; a successful ACK does not guarantee the final order state, and clients should confirm through order status lookups or order updates
- Classic spot private WS place-order and cancel-order use per-arg metadata:
  - `op: "trade"`
  - `args[0].id`
  - `args[0].instType`
  - `args[0].instId`
  - `args[0].channel`
  - `args[0].params`
- Classic contract private WS place-order and cancel-order follow the same overall request pattern as classic spot, but with futures-specific fields
- Current official docs found during design clearly cover Classic spot place/cancel and Classic contract place/cancel; they do not give the same clear coverage for Classic modify-order over WS

Official references used for this design:

- UTA WS place order: https://www.bitget.com/api-doc/uta/websocket/private/Place-Order-Channel
- UTA WS modify order: https://www.bitget.com/api-doc/uta/websocket/private/Modify-Order-Channel
- Classic contract WS place order: https://www.bitget.com/api-doc/classic/contract/websocket/private/Place-Order-Channel
- Classic spot WS place order: https://www.bitget.com/api-doc/spot/websocket/private/Place-Order-Channel
- Classic spot WS cancel order: https://www.bitget.com/api-doc/spot/websocket/private/Cancel-Order-Channel

Inference from the docs:

- UTA and Classic are close enough to share one socket lifecycle implementation.
- They are not close enough to share one message schema.

## Recommended Architecture

### SDK Layer

Keep one `PrivateWSClient`, but extend it with two distinct responsibilities:

1. subscription lifecycle for `WatchOrders` / `WatchPositions`
2. request/response trade RPC for order operations

Add these internal pieces:

- `pendingRequests map[string]chan []byte`
- `pendingMu sync.Mutex`
- request timeout handling
- helper to route responses by request ID

The transport layer should stay message-shape-agnostic except for extracting correlation IDs from supported response envelopes.

### Request Builders

Add protocol-specific builders and decoders instead of one universal request struct.

Recommended files:

```text
bitget/sdk/private_ws.go
bitget/sdk/private_ws_trade_uta.go
bitget/sdk/private_ws_trade_classic.go
```

Responsibilities:

- `private_ws.go`
  - connection lifecycle
  - login
  - subscribe / unsubscribe
  - pending request registry
  - response dispatch
  - generic `sendRequest(id, payload)` helper
- `private_ws_trade_uta.go`
  - build UTA place / cancel / modify payloads
  - decode UTA trade ACK responses
- `private_ws_trade_classic.go`
  - build classic spot place / cancel payloads
  - build classic contract place / cancel payloads
  - decode classic trade ACK responses

This keeps the protocol split below the adapter line.

## Adapter Routing Design

Keep the existing adapter API unchanged.

Routing rule:

- `OrderModeREST`
  - preserve current behavior exactly
- `OrderModeWS`
  - use private-profile WS order operations

Add lightweight connectivity guards:

- `Adapter.WsOrderConnected(ctx)` for perp
- `SpotAdapter.WsOrderConnected(ctx)` for spot

These should:

- require private credentials
- ensure the private WS client is connected and logged in
- not subscribe to order streams by themselves

This mirrors the useful part of Binance’s pattern without copying Binance’s wire protocol.

## Profile Responsibilities

### UTA Profiles

Change `utaPerpProfile` and `utaSpotProfile` so:

- `PlaceOrder`
  - use REST in REST mode
  - use UTA WS trade in WS mode
- `CancelOrder`
  - use REST in REST mode
  - use UTA WS trade in WS mode
- `ModifyOrder`
  - use REST in REST mode
  - use UTA WS trade in WS mode

`CancelAllOrders` should remain REST in v1 of this feature. This avoids pretending we support a WS channel we have not verified.

After a successful WS ACK:

- `PlaceOrder` should return a minimal `exchanges.Order`, like the current REST path
- `ModifyOrder` should re-query with `FetchOrderByID` to normalize final adapter output
- `CancelOrder` can return `nil` on successful ACK

### Classic Profiles

Mirror the same routing pattern in `classicPerpProfile` and `classicSpotProfile`, but only for the channels currently confirmed from docs:

- Classic spot:
  - `PlaceOrder` can use WS in `OrderModeWS`
  - `CancelOrder` can use WS in `OrderModeWS`
- Classic perp:
  - `PlaceOrder` can use WS in `OrderModeWS`
  - `CancelOrder` can use WS in `OrderModeWS`

Classic `ModifyOrder` should remain on its current REST path in v1 of this feature unless a verified Classic modify-order WS channel is confirmed during implementation.

Because Classic contract and Classic spot use different field sets, the SDK should expose separate helpers instead of one overloaded `PlaceOrderWS`.

## Request/Response Correlation Rules

Bitget WS trading must not assume responses arrive in order.

Rules:

1. Every WS trade request must generate a unique request ID.
2. The pending-request entry must be installed before the message is written.
3. Pending entries must be removed on timeout, write failure, or response receipt.
4. Response correlation must support both:
   - top-level `id`
   - nested response IDs inside the trade response channel payload used by Classic
5. Unknown IDs should be ignored, not treated as fatal

Timeout should initially match the existing private WS connect/login expectations: 10 seconds is acceptable for v1.

## Error Semantics

The adapter should preserve the current semantics:

- missing credentials: current private-access error path
- transport write/timeout/login failures: return as direct errors
- exchange-level ACK failure: return a descriptive SDK error including code and message

For UTA modify specifically:

- ACK success is not proof of final success
- the profile should continue to normalize by querying current order state after ACK, matching the repository’s existing expectation for `ModifyOrder`

## Test Strategy

Follow TDD at the seam where behavior changes.

Add unit tests for:

- `OrderModeWS` routing in UTA profiles
- `OrderModeWS` routing in Classic profiles
- private WS request correlation for:
  - top-level `id`
  - nested arg-style ID
- timeout cleanup of pending requests
- response decoding for UTA trade ACKs
- response decoding for classic trade ACKs

Avoid requiring live sockets in unit tests; use in-memory message dispatch where possible.

After package tests pass, run targeted Bitget live validation with `OrderModeWS` enabled for:

- spot orders
- perp orders
- order lifecycle

If Classic account permissions or Bitget WS trading permissions block live tests, report that explicitly instead of silently falling back to REST.

## Non-Negotiable Constraints

1. Do not change the default order mode.
2. Do not silently fall back from WS mode to REST for order placement or single-order cancellation.
3. Do not move account-mode branching out of private profiles into top-level adapter methods.
4. Do not claim WS support for `CancelAllOrders` unless the exact Bitget WS path is implemented and verified.
5. Do not claim Classic WS modify support without a verified channel and response contract.
6. Do not let subscription message handling and trade ACK handling race on shared response IDs.

## Implementation Outline

1. Extend `PrivateWSClient` with pending-request infrastructure and ID extraction.
2. Add UTA trade request/response helpers.
3. Add Classic trade request/response helpers.
4. Add `WsOrderConnected` helpers on perp and spot adapters.
5. Route `PlaceOrder` / `CancelOrder` / `ModifyOrder` by `OrderMode` inside each profile.
6. Add targeted tests.
7. Run package tests.
8. Run targeted Bitget live tests in WS mode.

## Risks

- Bitget may require special account-side permission enablement for WS trading; some docs explicitly mention contacting BD/RM.
- UTA modify-order is asynchronous, so a successful ACK alone is not enough.
- Classic spot and classic contract response shapes may diverge more than the public docs imply.

These are acceptable risks as long as the implementation keeps REST stable, keeps WS mode explicit, and reports permission failures honestly.
