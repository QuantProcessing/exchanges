---
name: adding-exchange-adapters
description: Use when adding a new exchange package or wiring a new exchange into the QuantProcessing exchanges Go repository, especially when deciding package structure, support level, private-stream readiness, and shared testsuite coverage.
---

# Adding Exchange Adapters

## Overview

This skill complements `exchanges`; it does not replace it. Load `exchanges` first for the shared contracts and invariants, then use this skill for adapter-specific routing: peer selection, capability classification, `sdk/` boundaries, private-support claims, and completion gates.

Keep mechanics in the reference files. Do not turn this entrypoint into a second copy of repo-wide interface docs.

## Before You Write Code

1. Read `exchanges`, then choose the nearest peer for each concern with market coverage first and auth model second.
2. One package may be the best match for `options.go` and `register.go`, while another is a better match for orderbook sync, private streams, or `adapter_test.go`. Borrow per concern; do not cargo-cult one package wholesale.
3. Classify the target before choosing file layout:
   - `public-data-only`
   - `trading-capable`
   - `lifecycle-capable`
   - `local-state-capable`
4. Load only the references needed for that capability level.

If you skip peer selection or capability classification, stop. You are guessing.

## Capability Routing

| Capability | Shared suites to wire in `adapter_test.go` | Minimum support claim | Load these references |
|------------|--------------------------------------------|-----------------------|-----------------------|
| `public-data-only` | `RunAdapterComplianceTests` | Private/account/trading surfaces return `exchanges.ErrNotSupported` | `references/live-test-wiring.md` |
| `trading-capable` | `RunAdapterComplianceTests`, `RunOrderSuite`, `RunOrderQuerySemanticsSuite` | Real trading and order-query behavior; unsupported shared surfaces return `exchanges.ErrNotSupported` | `references/order-semantics.md`, `references/live-test-wiring.md` |
| `lifecycle-capable` | `RunAdapterComplianceTests`, `RunOrderSuite`, `RunOrderQuerySemanticsSuite`, `RunLifecycleSuite` | Real `WatchOrders`; lifecycle claims are not valid without it | `references/order-semantics.md`, `references/private-streams-and-localstate.md`, `references/live-test-wiring.md` |
| `local-state-capable` | `RunAdapterComplianceTests`, `RunOrderSuite`, `RunOrderQuerySemanticsSuite`, `RunLocalStateSuite`; also `RunLifecycleSuite` if lifecycle correctness is claimed | `FetchAccount` plus a real `WatchOrders`; `WatchPositions` is additive, not the gate | `references/order-semantics.md`, `references/private-streams-and-localstate.md`, `references/live-test-wiring.md` |

`FetchOrderByID`, `FetchOrders`, and `FetchOpenOrders` are separate contracts. Read `references/order-semantics.md` before implementing any of them. Do not invent adapter-level history-order APIs or other order-query surfaces that do not exist on the current shared interface; if the shared interface cannot express a behavior yet, stop at the shared boundary and evolve the skill later.

## Architecture Decisions

Create a dedicated `sdk/` layer when any of these are true:

- the exchange needs request signing or auth token management
- low-level REST APIs split across multiple surface groups
- low-level WebSocket handling needs multiple connections or non-trivial session logic
- exchange-native wire structs would otherwise leak into adapter files
- spot and perp adapters will share reusable low-level clients or mappers

Choose the `sdk/` shape from the nearest peer:

- flat `sdk/` when one compact client shape covers the exchange
- `sdk/perp` plus `sdk/spot` when low-level APIs diverge materially by market
- shared helpers only when there is real reuse

Forbidden adapter-layer responsibilities:

- raw REST path building
- signing logic
- wire-format request or response structs
- WebSocket connection lifecycle internals

Use `references/sdk-boundaries.md` for the concrete split. Borrow the nearest peer's layout for the concern you are solving; do not invent a canonical layout or mirror one package wholesale.

## Private Support Matrix

| Claim | Required | Optional / additive | If unsupported |
|-------|----------|---------------------|----------------|
| Public data only | No private surfaces | None | Return `exchanges.ErrNotSupported` on every shared private surface |
| Trading capable | Real private REST/account or order surfaces needed for the claim | Private streams may still be unsupported | Unsupported shared stream surfaces return `exchanges.ErrNotSupported` |
| Lifecycle capable | `WatchOrders` is required | `WatchPositions` may still be unsupported | Use `exchanges.ErrNotSupported` for any shared private surface the exchange truly does not support |
| Local-state capable | `FetchAccount` and `WatchOrders` are hard prerequisites | `WatchPositions` adds position coverage but is not required for current local-state readiness | `WatchPositions` may return `exchanges.ErrNotSupported` if the exchange lacks a usable position stream |

Hard rules:

- Before claiming account or trading support, account for all three explicitly:
  - how private REST authentication works
  - whether a private WebSocket exists and which claims depend on it
  - which balances, orders, and positions come from REST snapshots versus stream deltas
- `WatchOrders` is a hard prerequisite for local state. Without it, do not claim `local-state-capable`.
- `WatchPositions` is additive. It improves position coverage but is not the universal gate for local state.
- If a shared surface is unsupported, return `exchanges.ErrNotSupported`; never return success while doing nothing.

Use `references/private-streams-and-localstate.md` for stream, snapshot, and local-state mechanics.

## Live Test Readiness

A new adapter is not integrated until `adapter_test.go` wires the intended shared suites and the live test inputs are documented.

Minimum live-test expectations:

- update `.env.example` with exchange-specific credentials and test symbols
- use stable symbols and quote defaults that match the adapter's real options
- use `internal/testenv` for repo-root `.env` loading and `RUN_FULL` / `RUN_SOAK` gate control instead of adding a new ad hoc lookup helper
- use clear skips when credentials, symbols, or unsupported capabilities are missing

Use `references/live-test-wiring.md` for the exact env-var and `testsuite` wiring pattern.

## Do Not Ship If

Stop immediately if any of these are true:

- you did not choose peers by market coverage first and auth model second, or you copied one package wholesale without re-evaluating each concern
- the adapter capability level is undefined or the wired `testsuite` coverage does not match it
- the registry advertises a market with no real adapter behind it
- `WatchOrders` is missing but lifecycle or local-state support is claimed
- `WatchPositions` is treated as required everywhere instead of additive where unsupported
- unsupported shared surfaces return no-op success instead of `exchanges.ErrNotSupported`
- `FetchOrderByID` is implemented by scanning only open orders
- adapter files own signing, raw REST construction, wire structs, or WebSocket lifecycle internals that belong in `sdk/`
- stream methods report success while doing nothing
- local orderbook is claimed as supported but never reaches a non-`nil` synced state
- live test prerequisites are undocumented, or `adapter_test.go` is missing

If you hit one of these gates, stop and fix the design before adding more code.
