# Adding Exchange Adapters

This document is the repository-owned guide for adding a new exchange package or expanding adapter capability in this codebase.

Use it for:

- new perp or spot adapters
- support-level upgrades such as public-only to trading-capable
- private stream and local-state readiness decisions
- live `testsuite` wiring and completion gates

This guide replaces the older repo-local `adding-exchange-adapters` skill as the maintained source of truth.

## Before You Write Code

1. Read `exchange.go`, `models.go`, `errors.go`, `utils.go`, `registry.go`, and the relevant `testsuite/*` files.
2. Choose the nearest existing peer by market coverage first and auth model second.
3. Borrow structure per concern instead of copying one package wholesale. One peer may be right for `options.go` and `register.go`, while another is better for orderbook sync or private streams.
4. Classify the target before choosing file layout:
   - `public-data-only`
   - `trading-capable`
   - `lifecycle-capable`
   - `local-state-capable`

If peer selection or capability classification is missing, stop and do that first.

## Capability Matrix

| Capability | Shared suites to wire in `adapter_test.go` | Minimum support claim |
|------------|--------------------------------------------|-----------------------|
| `public-data-only` | `RunAdapterComplianceTests` | Private/account/trading surfaces return `exchanges.ErrNotSupported` |
| `trading-capable` | `RunAdapterComplianceTests`, `RunOrderSuite`, `RunOrderQuerySemanticsSuite` | Real trading and order-query behavior; unsupported shared surfaces return `exchanges.ErrNotSupported` |
| `lifecycle-capable` | `RunAdapterComplianceTests`, `RunOrderSuite`, `RunOrderQuerySemanticsSuite`, `RunLifecycleSuite` | Real `WatchOrders`; lifecycle claims are not valid without it |
| `local-state-capable` | `RunAdapterComplianceTests`, `RunOrderSuite`, `RunOrderQuerySemanticsSuite`, `RunLocalStateSuite`, and `RunLifecycleSuite` when lifecycle support is claimed | `FetchAccount` plus a real `WatchOrders`; `WatchPositions` is additive, not the gate |

`FetchOrderByID`, `FetchOrders`, and `FetchOpenOrders` are separate contracts. Preserve that distinction in both implementation and tests.

## Architecture And `sdk/` Boundaries

Create a dedicated `sdk/` layer when any of these are true:

- the exchange needs signing or auth token management
- low-level REST APIs split across multiple surface groups
- WebSocket handling needs multiple connections or non-trivial session logic
- exchange-native wire structs would otherwise leak into adapter files
- spot and perp adapters will share reusable low-level clients or mappers

Choose the `sdk/` shape from the nearest peer:

- flat `sdk/` when one compact client shape covers the exchange
- `sdk/perp` plus `sdk/spot` when low-level APIs diverge materially by market
- shared helpers only when there is real reuse

Adapter-layer responsibilities:

- symbol mapping and validation
- picking the low-level method that satisfies the shared interface
- mapping validated SDK responses into unified `exchanges.*` models
- adapter-side policy such as slippage handling and `ErrNotSupported`

Do not keep these in adapter files:

- raw REST path building
- signing logic
- wire-format request or response structs
- WebSocket connection lifecycle internals

Rule of thumb: `sdk/` speaks exchange-native protocol, adapters speak the shared repository contract.

## Order Contract Rules

The current shared order-query surfaces are:

- `FetchOrderByID`
- `FetchOrders`
- `FetchOpenOrders`

Required behavior:

- use base symbols at the adapter boundary
- filter by symbol when the low-level API returns broader results
- return unified `exchanges.Order` values
- return `exchanges.ErrOrderNotFound` when lookup is exhausted

Hard rules:

- do not implement `FetchOrderByID` by scanning only open orders and pretend that is complete behavior
- do not collapse `FetchOrders` into `FetchOpenOrders` unless the exchange truly exposes only open orders and that limitation is explicit in code and tests
- do not invent adapter-only history APIs beyond the shared interface
- do not return `nil, nil` for missing orders

Preferred lookup order for `FetchOrderByID`:

1. dedicated order-detail endpoint
2. authenticated order history or broader order-list endpoint that includes terminal states
3. symbol-scoped history/list endpoint that can still resolve the order

## Private Streams And Local State

`LocalState` depends on adapter behavior. It is not a separate adapter interface.

Practical readiness rules:

- `WatchOrders` is mandatory for lifecycle claims
- `WatchOrders` is mandatory for local-state readiness
- `WatchPositions` is additive coverage, not the universal gate
- unsupported shared private surfaces must return `exchanges.ErrNotSupported`, not no-op success

Responsibility split:

1. `FetchAccount` establishes the initial coherent snapshot
2. `WatchOrders` supplies order lifecycle deltas
3. `WatchPositions` supplies position deltas when supported
4. `LocalState` owns caching, fan-out, and periodic reconciliation

If the exchange lacks a usable private order stream, do not claim `lifecycle-capable` or `local-state-capable`.

## Live Test Wiring

Live adapter integration is incomplete until `adapter_test.go` wires the shared `testsuite` coverage for the claimed support level.

Required wiring:

- update the repository-root `.env.example`
- use `internal/testenv` for env loading and `RUN_FULL` / `RUN_SOAK` gating
- construct the real adapter from env-backed options
- wire the shared suite matrix that matches the adapter's capability level
- use clear skips only for missing credentials, missing symbols, or genuine exchange limitations

Environment variable guidance:

- use uppercase exchange-prefixed names
- keep auth names aligned with `options.go`
- add `*_PERP_TEST_SYMBOL` and `*_SPOT_TEST_SYMBOL` only for supported markets
- add `*_QUOTE_CURRENCY` only when the adapter exposes quote configuration

Stable defaults matter:

- use liquid, durable symbols
- match the adapter's default quote from `options.go`
- avoid ephemeral listings or promo-only markets

## Do Not Ship If

Stop immediately if any of these are true:

- peer selection was skipped or one package was copied wholesale without re-evaluating each concern
- the adapter capability level is undefined or the wired `testsuite` coverage does not match it
- the registry advertises a market type with no real adapter behind it
- `WatchOrders` is missing but lifecycle or local-state support is claimed
- unsupported shared surfaces return no-op success instead of `exchanges.ErrNotSupported`
- `FetchOrderByID` is implemented by scanning only open orders
- adapter files own signing, raw REST construction, wire structs, or WebSocket lifecycle internals that belong in `sdk/`
- stream methods report success while doing nothing
- local orderbook is claimed as supported but never reaches a non-`nil` synced state
- live test prerequisites are undocumented or `adapter_test.go` is missing

## Reference Files

Start from the smallest authoritative set:

- `exchange.go`
- `models.go`
- `errors.go`
- `utils.go`
- `registry.go`
- `local_state.go`
- `testsuite/compliance.go`
- `testsuite/order_suite.go`
- `testsuite/lifecycle_suite.go`
- `testsuite/helpers.go`
- `<exchange>/options.go`
- `<exchange>/register.go`
- `<exchange>/perp_adapter.go`
- `<exchange>/spot_adapter.go`
- `<exchange>/adapter_test.go`

For final review before merge, also use `docs/superpowers/checklists/exchange-adapter-review.md`.
