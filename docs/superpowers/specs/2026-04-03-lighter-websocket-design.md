# Lighter WebSocket Alignment Design

## Status

Proposed and approved for implementation.

## Problem

The current Lighter WebSocket implementation in this repository no longer matches the exchange's current protocol closely enough to be treated as reliable infrastructure.

The most important mismatches are:

- the SDK assumes text JSON only, while the production endpoint also supports `encoding=msgpack`
- the transport layer uses an application-level JSON ping loop instead of WebSocket ping frames
- the order book callback path does local 100ms polling in one adapter even though the exchange now batches order book updates every 50ms
- the order book continuity path is incomplete: nonce gaps are detected, but the adapter does not fully resubscribe and recover from them
- several official channels and newer payload fields are missing from the local SDK type system
- spot and perp adapters do not expose streaming behavior consistently with the current exchange protocol

This is not only an order book issue. The underlying WebSocket client, typed message model, and adapter integration all need to be brought back into alignment.

## Goals

- Align the local Lighter WebSocket SDK with the current official protocol.
- Support both JSON and MessagePack transport encodings.
- Use protocol-appropriate keepalive behavior.
- Normalize channel parsing and typed event dispatch inside `lighter/sdk`.
- Keep the public adapter interfaces stable where practical.
- Make order book updates event-driven instead of locally polled.
- Handle Lighter order book continuity and resubscription correctly.
- Add coverage for new official channels and recently added payload fields.
- Preserve repository architecture conventions by keeping exchange-specific order book continuity logic in the Lighter order book and adapter layer rather than the generic WebSocket client.

## Non-Goals

- Do not redesign the repository-wide streaming interfaces.
- Do not force all new Lighter channels into the unified `exchanges.Exchange` layer immediately.
- Do not move exchange-specific order book sync rules into shared repository abstractions.
- Do not perform a full SDK API rewrite.
- Do not change unrelated Lighter REST behavior in this phase.

## Confirmed Product Decisions

These decisions were explicitly confirmed during discussion and are fixed for this design:

- this is a full Lighter WebSocket alignment effort, not a single-feature patch
- light breaking changes are acceptable inside `lighter/sdk` if the adapter-facing surface remains mostly stable
- order book continuity handling should follow repository precedent and stay in the exchange-specific order book plus adapter path
- the transport layer should support `encoding=msgpack` through the WebSocket URL query string
- `permessage-deflate` should be handled by the WebSocket implementation rather than by custom application code
- spot `WatchTicker` should be upgraded from unsupported to supported

## Current State Summary

### Transport layer gaps

`lighter/sdk/ws_client.go` currently:

- connects to a fixed URL with no query-parameter transport configuration
- assumes all inbound payloads are text JSON
- writes outbound messages with `WriteJSON` only
- uses an application-level `{"type":"ping"}` loop every 30 seconds
- does not distinguish transport errors from protocol decoding errors

### Message model gaps

`lighter/sdk/ws_types.go` and `lighter/sdk/types.go` do not fully reflect current payloads. Examples include:

- `ticker` channel support is missing
- `spot_market_stats` channel support is missing
- `account_all_assets`, `account_spot_avg_entry_prices`, `pool_data`, and `pool_info` are missing
- newer fields such as `last_updated_at`, `liquidation_trades`, `total_discount`, extended trade identifiers, additional fee fields, and volume fields are incomplete or absent

### Adapter behavior gaps

Current adapter behavior diverges from the protocol:

- perp `WatchOrderBook` performs 100ms polling snapshots from the local book instead of emitting on each applied exchange update
- spot and perp order book watch paths are not aligned with each other
- perp `WatchTicker` is driven only by `market_stats` instead of combining the exchange's ticker and stats channels
- spot `WatchTicker` is still marked unsupported

## Design Principles

1. Transport concerns and exchange business semantics should be separated.
2. Generic WebSocket infrastructure should be protocol-aware but exchange-agnostic.
3. Exchange-specific state continuity should stay in exchange-specific order book code.
4. Reconnection and resubscription should be automatic at the client layer.
5. Resynchronization after order book continuity loss should be orchestrated by the adapter layer.
6. Existing adapter-facing APIs should remain stable unless a change materially improves correctness.

## Considered Approaches

### Option 1: Minimal patching

Keep the current raw-byte client, add msgpack decoding, patch keepalive, and fix order book callbacks in place.

Why not chosen:

- leaves the client architecture fragile
- keeps parsing duplicated across adapters
- makes future Lighter protocol updates harder to absorb

### Option 2: Rebuild the WebSocket core while preserving adapter-facing behavior

Refactor `lighter/sdk` into a configurable transport plus typed dispatch layer, while keeping adapter public methods mostly stable.

Why chosen:

- addresses protocol drift comprehensively
- contains breaking changes inside `lighter/sdk`
- matches the repository's preferred balance between correctness and stability

### Option 3: Full high-level streaming rewrite

Turn the Lighter SDK into a stateful high-level stream hub that owns book and account state directly.

Why not chosen:

- exceeds the agreed scope
- would create more adapter churn than needed for this upgrade

## Chosen Architecture

The final design has two main layers:

1. A rebuilt protocol-aware `lighter/sdk` WebSocket client.
2. Exchange-specific adapter and local-order-book logic for continuity-sensitive state such as order books.

## WebSocket Client Design

### Client configuration

`lighter/sdk/ws_client.go` should support an explicit client configuration model, including:

- base WebSocket URL
- `readonly` query flag
- encoding mode: `json`, `msgpack`, or `auto`
- keepalive interval
- reconnect backoff settings
- logger and error callback

The default mainnet URL remains `wss://mainnet.zklighter.elliot.ai/stream`, but the client must be able to append transport query parameters such as:

- `readonly=true`
- `encoding=msgpack`

For this design, `encoding=msgpack` is treated as a supported production transport mode based on confirmed endpoint behavior observed during discussion.

### Transport handling

Inbound frame handling should be:

- text frame -> decode as JSON
- binary frame -> decode as MessagePack
- `encoding=auto` -> trust actual frame type rather than only the configured preference

Outbound handling should remain JSON for subscription, unsubscribe, `pong`, and transaction messages unless Lighter explicitly requires a binary outbound format in the future.

### Compression

The server advertises `permessage-deflate`. No application-level compression layer should be added. The WebSocket library should negotiate and process compression transparently.

### Keepalive

The client should keep the socket alive with real WebSocket ping control frames on a safe interval below the server's 2-minute timeout.

Additionally:

- if the server sends an application-level `{"type":"ping"}` message, the client should continue replying with `{"type":"pong"}` for compatibility
- any successful outbound application message also counts as activity, but ping frames remain the default keepalive mechanism

### Message normalization

The client should normalize inbound payloads into a common envelope shape before dispatch. The normalized envelope should capture at least:

- `Type`
- `Channel`
- `Timestamp`
- `LastUpdatedAt`
- normalized payload content
- raw message bytes for logging/debugging

This allows both `subscribed/*` and `update/*` messages to pass through one dispatch path.

### Subscription registry

The current `channel -> raw handler` map should become a richer registry entry containing:

- channel name
- auth token when required
- original subscribe request
- callback registrations
- whether the subscription should be replayed after reconnect

This registry is used for reconnect recovery only. It must not encode exchange-specific resync semantics such as order book gap recovery.

### Dispatch model

The SDK should support:

- typed callbacks for known message families
- a compatibility raw callback path for existing adapter code

Raw compatibility callbacks should receive normalized JSON bytes even if the underlying frame was MessagePack. That preserves most existing adapter parsing code while allowing the transport to move forward.

### Reconnect behavior

If the connection drops unexpectedly:

- the client reconnects with exponential backoff
- once connected, it replays active subscriptions
- it does not attempt to infer channel-specific state repair beyond restoring subscriptions

This matches the repository pattern where the generic transport restores connectivity and the exchange-specific logic repairs state continuity when needed.

## Channel Coverage

The SDK should expose typed support for the official Lighter channels currently relevant to the repository:

- `order_book/{MARKET_INDEX}`
- `ticker/{MARKET_INDEX}`
- `market_stats/{MARKET_INDEX}`
- `market_stats/all`
- `spot_market_stats/{MARKET_INDEX}`
- `spot_market_stats/all`
- `trade/{MARKET_INDEX}`
- `height`
- `account_all/{ACCOUNT_ID}`
- `account_market/{MARKET_ID}/{ACCOUNT_ID}`
- `user_stats/{ACCOUNT_ID}`
- `account_tx/{ACCOUNT_ID}`
- `account_all_orders/{ACCOUNT_ID}`
- `account_orders/{MARKET_INDEX}/{ACCOUNT_ID}`
- `account_all_trades/{ACCOUNT_ID}`
- `account_all_positions/{ACCOUNT_ID}`
- `account_all_assets/{ACCOUNT_ID}`
- `account_spot_avg_entry_prices/{ACCOUNT_ID}`
- `pool_data/{ACCOUNT_ID}`
- `pool_info/{ACCOUNT_ID}`
- `notification/{ACCOUNT_ID}`

Not all of these channels need adapter-level integration immediately, but `lighter/sdk` should understand them and provide typed entry points.

## Type Model Updates

`lighter/sdk/ws_types.go` and related SDK types should be expanded to match current payload shapes, including:

- `last_updated_at` fields for order book and ticker data
- `liquidation_trades` on trade updates
- extended order fields such as integrator fee collector and integrator fee values
- extended trade identifiers such as `trade_id_str`, `ask_id_str`, `bid_id_str`, `ask_client_id_str`, `bid_client_id_str`
- `transaction_time`, account PnL fields, and optional fee/margin fields on trades
- `total_discount` on positions
- volume fields on account all trades
- asset maps for account-all-assets
- average-entry-price payloads for spot account assets
- newer pool information fields such as `sharpe_ratio`, `daily_returns`, `share_prices`, and strategies

The type system should tolerate omitted optional fields without treating them as protocol errors.

## Order Book Design

### Responsibility split

The generic WebSocket client is responsible for:

- receiving messages
- decoding JSON or MessagePack
- routing `order_book` messages to the subscription callback

The Lighter local order book implementation in `lighter/orderbook.go` is responsible for:

- recognizing initial full snapshot behavior
- tracking last applied nonce
- validating `begin_nonce == last_nonce` for continuity
- reporting when continuity is lost
- preserving the last known good book while a resync is in progress

The adapter layer is responsible for:

- reacting to order book continuity errors
- orchestrating unsubscribe and resubscribe
- deciding when callbacks may emit data

This matches the repository pattern used by Binance in this codebase, where exchange-specific order book synchronization rules live alongside the exchange-specific order book implementation and adapter orchestration.

### Order book state machine

The Lighter local order book should track at least these logical states:

- `cold`: subscribed but no usable snapshot yet
- `ready`: snapshot received and continuity preserved
- `resyncing`: a nonce gap or invalid sequence was detected and the adapter is resubscribing for a new snapshot

Behavior:

- the first full snapshot initializes the book and marks it ready
- each incremental update is applied only if its `begin_nonce` matches the previous `nonce`
- if a gap is detected, the order book returns a typed resync-required error
- the last known good depth remains readable during resync, but no new watch callbacks are emitted until the replacement snapshot is ready

### Resynchronization method

Unlike Binance, Lighter does not need a separate REST snapshot repair path for this design. The exchange sends a full snapshot when the order book channel is subscribed.

Therefore the adapter recovery path is:

1. detect continuity failure from `lighter/orderbook.go`
2. unsubscribe from `order_book/{MARKET_INDEX}`
3. subscribe again to `order_book/{MARKET_INDEX}`
4. wait until the new snapshot marks the book ready
5. resume event-driven callback emission

## Adapter Changes

### Order book watches

`lighter/perp_adapter.go` and `lighter/spot_adapter.go` should:

- share the same event-driven order book watch behavior
- stop using local 100ms polling callbacks
- emit one callback per successfully applied exchange batch update
- coordinate resubscribe-driven repair when the local order book reports a gap

### Ticker watches

Perp:

- `WatchTicker` should combine `ticker/{MARKET_INDEX}` and `market_stats/{MARKET_INDEX}`
- `ticker` supplies best bid, best ask, and immediate book-derived prices
- `market_stats` supplies last trade price, volume, daily high, daily low, index price, and mark price

Spot:

- `WatchTicker` should become supported
- the primary feed should be `spot_market_stats/{MARKET_INDEX}`
- if a spot-specific best-bid-offer channel is unavailable, `Bid` and `Ask` may remain empty rather than forcing a hidden order book subscription

### Trade, order, fill, and position streams

Existing adapter semantics should remain intact as much as possible, but they should consume typed SDK messages rather than reparsing raw transport payloads everywhere.

This includes:

- `WatchTrades`
- `WatchOrders`
- `WatchFills`
- `WatchPositions`

### Unsupported-path tests

Because spot `WatchTicker` becomes supported, `lighter/unsupported_test.go` must be updated accordingly.

## Error Handling

The WebSocket layer should distinguish:

- transport errors
- decode errors
- protocol-shape errors
- subscription replay errors

Errors should include enough context to debug live incidents, especially:

- channel
- message type
- negotiated or detected encoding
- a safe raw-message summary

Order book resync requests are not generic transport errors. They should be modeled as an exchange-specific continuity outcome surfaced by the Lighter order book implementation.

## Testing Plan

### Unit tests for transport and dispatch

Add coverage for:

- JSON text frame decoding
- MessagePack binary frame decoding
- mixed-mode `encoding=auto`
- typed dispatch by `type` and `channel`
- reconnect subscription replay
- WebSocket ping keepalive behavior
- compatibility raw callback output after MessagePack decode

### Unit tests for Lighter order book logic

Add dedicated tests for `lighter/orderbook.go` covering:

- first snapshot initialization
- valid incremental continuation
- gap detection through mismatched `begin_nonce`
- preserving the last good snapshot during resync
- ready-state transitions after resubscribe-driven reinitialization

### Adapter behavior tests

Add or adjust tests for:

- perp and spot order book watches emitting on exchange-driven events rather than local polling timers
- adapter-triggered order book resubscribe after continuity loss
- perp ticker assembly from ticker plus market-stats channels
- spot ticker support from spot market stats
- updated unsupported-path expectations

### Live verification

After implementation, rerun existing live or compliance coverage centered on:

- `WatchOrderBook`
- `WatchTicker`
- `WatchTrades`
- `WatchFills`

for both spot and perp adapters where credentials and environment permit.

## Risks And Mitigations

### Risk: Message type drift between docs and live payloads

Mitigation:

- treat both `subscribed/*` and `update/*` as valid where channel semantics allow
- keep decode logic tolerant of extra fields

### Risk: Reconnect replays subscriptions but business state is still stale

Mitigation:

- keep reconnect logic generic
- keep exchange-specific resync logic in adapter paths, especially for order books

### Risk: MessagePack support breaks existing raw handlers

Mitigation:

- preserve raw callback compatibility by feeding normalized JSON bytes after binary decode

### Risk: Spot ticker semantics are incomplete

Mitigation:

- explicitly support partial ticker population
- avoid inventing best bid and ask if the exchange feed does not provide them directly

## Implementation Direction

Recommended rollout order:

1. Refactor `lighter/sdk/ws_client.go` into a configurable transport plus normalized dispatch client.
2. Expand `lighter/sdk/ws_types.go` and related SDK types to cover the current official payloads.
3. Add subscription helpers for missing official channels.
4. Rework `lighter/orderbook.go` into a continuity-aware local order book with explicit resync signaling.
5. Refactor perp and spot `WatchOrderBook` to use event-driven callbacks plus unsubscribe/resubscribe repair.
6. Upgrade perp and spot ticker watch paths to use the appropriate official channels.
7. Update tests, then run live verification where possible.

## Open Questions Resolved

Should order book continuity handling live in the generic WebSocket client?

- No. It should live in `lighter/orderbook.go` and the Lighter adapters, consistent with repository precedent.

Should this project only patch order book support?

- No. The design explicitly covers the whole Lighter WebSocket surface currently relevant to this repository.

Should MessagePack support be treated as optional?

- No. It should be implemented as a first-class transport option because the production endpoint supports `encoding=msgpack`.
