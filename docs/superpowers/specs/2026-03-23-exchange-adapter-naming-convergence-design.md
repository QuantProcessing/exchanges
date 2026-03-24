# Exchange Adapter Naming Convergence Design

## Status

Draft for repository-wide naming convergence after the adapter-layering rollout.

## Problem

The adapter-layering rollout standardized constructor semantics, transport classification, and stable error behavior across the repository, but two naming drifts remain unresolved:

- SDK query and trading methods still expose a small number of legacy names that differ from the repository-preferred verbs.
- several SDK websocket base clients still use `WsClient` while other packages already use `WSClient`, `PublicWSClient`, or `PrivateWSClient`.

These drifts do not currently break behavior, but they continue to hurt cross-package readability and make it harder to treat adapter packages as variations of one repository design.

## Goal

Establish one repository-preferred naming scheme for the most visible SDK entrypoints without forcing a breaking cleanup in one pass.

This document is an amendment to the adapter-layering baseline, not a parallel naming policy. It narrows two previously deferred naming questions into one implementable pass and explicitly leaves the rest deferred.

This design is intentionally narrow:

- unify SDK query and private-order naming where there is already clear repository precedent
- unify websocket base-client casing where the same concept is implemented in multiple packages
- preserve compatibility for existing call sites through thin wrappers or aliases

## Non-Goals

- rename exchange-native request or response fields
- rename websocket channel names, RPC method names, or protocol payload keys
- rename local-orderbook methods such as `GetDepth`
- redesign package layout or revisit stream/file placement in this task
- normalize every exchange-specific client naming pattern such as `WebsocketClient`, `WsApiClient`, or `BaseWsClient`

## Scope

### In Scope

- repository-preferred SDK query and private-order naming, using Backpack as the already-landed precedent package
- `WsClient` to `WSClient` convergence in:
  - `aster/sdk/perp`
  - `aster/sdk/spot`
  - `binance/sdk/perp`
  - `binance/sdk/spot`
  - `okx/sdk`
  - `standx/sdk`
- spec, checklist, and gap-doc updates needed to mark these naming rules as landed

### Out Of Scope

- `grvt` and `hyperliquid` `WebsocketClient`
- `nado` `BaseWsClient` and `WsApiClient`
- `lighter` `WebsocketClient`
- local-orderbook naming
- any broader file-layout cleanup

## Repository Naming Rules

After this change, repository-preferred SDK naming is:

- SDK public query methods use `Get*`
- SDK private trading methods use `Place*`, `Cancel*`, and `Modify*`
- websocket base-client types that follow the generic `WsClient` pattern use `WSClient`
- role-specific websocket clients use `PublicWSClient` and `PrivateWSClient` where those roles exist

The repository will continue to allow exchange-specific names when the concept is genuinely different. This task only changes names that represent the same concept and already have a clear preferred form elsewhere in the codebase.

This does not close the broader question of whether every exchange-specific websocket type in the repository should eventually converge on one shared family of names. After this pass:

- `WSClient` is the preferred casing for packages that currently expose the generic `WsClient` concept
- `WebsocketClient`, `BaseWsClient`, and `WsApiClient` remain tolerated exchange-specific exceptions
- the layering baseline should record those remaining families as deferred, with future convergence left to a separate task

## Compatibility Strategy

This design uses compatibility layers instead of a breaking rename.

### Websocket Client Types

For packages that currently define `WsClient`:

- `WSClient` becomes the primary concrete type definition
- `type WsClient = WSClient` remains as a compatibility alias
- `NewWSClient(...)` becomes the primary constructor
- `NewWsClient(...)` remains as a thin compatibility wrapper that forwards to `NewWSClient(...)`

Rules:

- the implementation must live in exactly one concrete type
- the compatibility alias must not duplicate methods
- new code, new tests, and new docs should reference `WSClient`

### Backpack Query And Trading Methods

For Backpack SDK methods:

- `GetOrderBook` becomes the primary public orderbook query method
- `GetDepth` remains as a thin compatibility wrapper
- `PlaceOrder` becomes the primary private order-placement method
- `ExecuteOrder` remains as a thin compatibility wrapper

Rules:

- the preferred method owns the implementation
- the legacy name is only a compatibility shim
- adapters, stubs, tests, and docs should be updated to call the preferred method

## Package-Level Design

### Backpack

Repository role:

- Backpack is not new implementation scope in this task.
- Backpack is the already-landed precedent for `GetOrderBook` and `PlaceOrder` as preferred SDK names.
- This task only verifies that the layering baseline, checklist, and Backpack gap doc consistently describe that landed state.

Do not change:

- `WSClient` naming, because Backpack already matches the preferred websocket casing
- local-orderbook `GetDepth`

### Aster

Change:

- rename the concrete websocket base client type from `WsClient` to `WSClient` in both `sdk/perp` and `sdk/spot`
- add `type WsClient = WSClient`
- add `NewWsClient(...)` compatibility wrappers that forward to `NewWSClient(...)`
- update internal references to prefer `WSClient` and `NewWSClient`

### Binance

Change:

- rename the concrete websocket base client type from `WsClient` to `WSClient` in both `sdk/perp` and `sdk/spot`
- add `type WsClient = WSClient`
- add `NewWsClient(...)` compatibility wrappers that forward to `NewWSClient(...)`
- update internal references to prefer `WSClient` and `NewWSClient`

### OKX

Change:

- rename the concrete websocket client type from `WsClient` to `WSClient`
- keep `type WsClient = WSClient` as a compatibility alias
- add `NewWsClient(...)` compatibility wrapper next to `NewWSClient(...)`
- update adapters, SDK files, and tests to prefer `WSClient`

### StandX

Change:

- rename the concrete websocket base client type from `WsClient` to `WSClient`
- keep `type WsClient = WSClient` as a compatibility alias
- add `NewWsClient(...)` compatibility wrapper next to `NewWSClient(...)`
- update SDK files to prefer `WSClient`

## Implementation Order

1. Align the layering baseline, Backpack gap doc, and checklist so Backpack is recorded as the landed precedent and websocket naming deferral is narrowed to the remaining non-`WsClient` families.
2. Run or refresh Backpack regression coverage only as verification for the already-landed `GetOrderBook` and `PlaceOrder` compatibility wrappers.
3. Update websocket base-client naming in Aster.
4. Update websocket base-client naming in Binance.
5. Update websocket base-client naming in OKX.
6. Update websocket base-client naming in StandX.
7. Update repository docs to mark the naming convergence pass as landed and keep the remaining non-`WsClient` naming families explicitly deferred.

## Verification

Minimum verification for the implementation phase:

- targeted Backpack SDK naming tests only as regression coverage for the already-landed compatibility wrappers
- targeted package tests for each renamed websocket-client package
- repository compile check

Compatibility verification is mandatory. The implementation plan must require at least one deterministic compatibility test for each renamed family:

- one compatibility test for Backpack `GetDepth -> GetOrderBook`
- one compatibility test for Backpack `ExecuteOrder -> PlaceOrder`
- one compatibility test per package family for `NewWsClient -> NewWSClient` or `WsClient -> WSClient`

Credential-dependent live tests are not required for this naming task.

## Risks

- type aliases can hide unintended duplicate definitions if the rename is not centralized carefully
- constructor wrappers can leave tests split across old and new names if call sites are not updated consistently
- docs can drift again if the spec is not updated in the same rollout as the code

## Acceptance Criteria

This design is complete when:

- the layering baseline explicitly treats Backpack as the landed precedent for `GetOrderBook` and `PlaceOrder`
- legacy Backpack names remain only as documented compatibility wrappers
- Aster, Binance, OKX, and StandX use `WSClient` as the primary websocket base-client type name
- legacy `WsClient` names remain only as compatibility aliases or wrappers
- new tests and updated internal call sites use the preferred names
- compatibility wrappers are covered by deterministic tests
- the adapter-layering spec no longer treats Backpack SDK query/order naming as deferred and narrows websocket naming deferral to the remaining non-`WsClient` families
