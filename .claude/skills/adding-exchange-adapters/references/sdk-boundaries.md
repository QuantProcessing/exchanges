# SDK Boundaries

Use the nearest peer `sdk/` layout that already matches the exchange's low-level API shape. Do not invent a canonical `sdk/` tree for this skill.

## Start With Peer Selection

Choose the closest existing package by low-level API shape:

- `backpack/sdk/`: flat layout, one compact client shape, shared private/public REST plus WS helpers
- `nado/sdk/` or `grvt/sdk/`: flat layout with richer WS and signing, still one shared market surface
- `binance/sdk/spot` plus `binance/sdk/perp`: split layout because low-level spot and perp APIs diverge materially

Mirror the nearest peer for the concern you are solving. One exchange may borrow `binance`-style market splits but `backpack`-style `adapter_test.go` wiring.

## What Belongs In `sdk/`

Put low-level exchange mechanics in `sdk/`:

- raw REST path and query construction
- request signing, auth headers, token refresh, listen-key management
- wire-format request and response structs
- WebSocket connection lifecycle, auth handshakes, reconnect, ping/pong, channel subscription payloads
- low-level methods named after exchange-native surfaces such as `GetOrder`, `GetOpenOrders`, `CreateOrder`, `OrderStatus`

## What Belongs In Adapter Files

Keep the adapter layer focused on the unified `exchanges` contract:

- `FormatSymbol` and `ExtractSymbol`
- choosing which low-level SDK method satisfies `FetchTicker`, `FetchOrder`, `WatchOrders`, and other shared methods
- mapping validated SDK responses into unified models such as `exchanges.Order`, `exchanges.Position`, and `exchanges.Account`
- shared adapter-side policy such as slippage handling, `ErrNotSupported`, and symbol-level filtering expected by `exchange.go`

If a line of code is about exchange-native transport details rather than the shared interface, it probably belongs in `sdk/`.

## Wire-Type Boundary

`sdk/` may expose exchange-native types internally. Adapter files should convert those types into unified models before data crosses the shared boundary.

Preferred flow:

1. `sdk/` returns a typed wire model or low-level result.
2. Adapter validates presence, status, and symbol assumptions.
3. Adapter maps into `exchanges.*` models.

Do not let `sdk/` wire structs leak upward into callbacks, shared helpers, or exported adapter APIs.

## Mapping Rules

Mapping helpers should sit at the seam between wire types and unified models:

- keep wire-type parsing close to the adapter or in adapter-local mapping helpers
- normalize symbols back to base symbols before returning unified models
- validate incomplete or ambiguous wire payloads before populating `exchanges.Order`
- use sentinel errors from [`errors.go`](/home/xiguajun/Documents/GitHub/Exchanges/.worktrees/skill-adding-exchange-adapters/errors.go) at the adapter boundary, not exchange-specific string matching in callers

Do not populate `exchanges.Order` or `exchanges.Position` directly from unvalidated JSON or raw maps.

## Share Or Split Spot And Perp SDK Code

Share SDK code when the low-level surfaces are genuinely the same:

- same auth and signing flow
- same REST client and transport middleware
- same WS session shape
- same wire types with minor market-specific flags

Split spot and perp SDK code when the low-level APIs differ enough that sharing would hide real divergence:

- different base URLs or product families
- different order/account endpoints
- different WS auth or event payloads
- different symbol or instrument identifiers

Use shared helpers only for real reuse such as signing, transport, or common wire utilities. Do not create `sdk/common` just to look tidy.

## Concrete Anti-Patterns

Avoid these:

- building REST URLs inline in `perp_adapter.go` or `spot_adapter.go`
- duplicating WS auth or reconnect logic across spot and perp adapters instead of sharing it in `sdk/`
- putting exchange-native request structs in adapter files
- returning exchange-native wire types from adapter methods
- forcing spot and perp into a shared `sdk/` when their low-level APIs are already separate
- splitting into `sdk/spot` and `sdk/perp` when one shared client would stay simpler and clearer
- mapping raw JSON directly into unified `exchanges.Order` without validation

The boundary is simple: `sdk/` speaks exchange-native protocol; adapters speak the unified repository contract.
