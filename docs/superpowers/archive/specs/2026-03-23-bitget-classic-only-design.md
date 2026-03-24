# Bitget Classic-Only Adapter Design

## Goal

Simplify the `bitget` adapter so it supports only Bitget Classic account mode for private capabilities while preserving the existing package name, registry key, public market-data support, classic REST trading, and classic WebSocket order transport.

This change intentionally removes all UTA-specific code paths, account-mode selection, and UTA-specific tests and docs.

## User-Confirmed Scope

The user explicitly wants:

- remove all UTA-related implementation from `bitget`
- remove all account-mode branching and detection logic
- keep `bitget` as the package name and `BITGET` as the registry key
- keep only the classic account implementation
- keep Bitget behavior conceptually similar to the repository's Binance adapters: one adapter package, one private API shape, no explicit multi-account abstraction

This design does not attempt to:

- add any new Bitget features
- preserve partial UTA scaffolding for future reuse
- rename the package or create `bitgetclassic`

## Why This Change Is Correct

The current branch has accumulated real complexity from supporting both Classic and UTA:

- private profiles split by account mode
- constructor-time account-mode detection and validation
- UTA-only private REST assumptions
- UTA-only private WebSocket trade helpers
- UTA-only tests and docs

The user has now explicitly decided that this complexity is unwanted and that Bitget should only support the stable classic path.

Given that instruction, the correct engineering move is deletion rather than further adaptation.

## Target Architecture

After this change, `bitget` should have exactly one private implementation family:

- classic spot
n- classic perp

Public functionality remains shared and unchanged where possible.

There should be no runtime account-mode branching.

### Adapter Layer

`bitget/perp_adapter.go` and `bitget/spot_adapter.go` should:

- keep public market-data behavior unchanged
- keep constructor names unchanged
- remove the `accountMode` field
- remove account-mode detection during initialization
- instantiate the classic private profile directly
- preserve the explicit default `OrderModeREST` behavior already restored in this branch

### Private Profile Layer

`bitget/private_profile.go` should stop being a mode switch.

It should remain only as a small constructor/helper boundary if useful, but:

- no `accountModeAuto`
- no `accountModeUTA`
- no `accountModeClassic`
- no mode-based branching

It should simply return:

- `classicPerpProfile` for perp
- `classicSpotProfile` for spot

### SDK Layer

Keep only the classic private transport helpers:

- `bitget/sdk/classic_private_rest.go`
- `bitget/sdk/private_ws.go`
- `bitget/sdk/private_ws_trade_classic.go`

Delete the UTA-specific WebSocket trade helper:

- `bitget/sdk/private_ws_trade_uta.go`

The low-level REST client may still contain generic `/api/v3` methods if they are needed by the remaining classic/public code, but any UTA-only helper that becomes dead code should be removed.

## Initialization Semantics

Initialization should become simpler:

- no credentials: allow public-only construction
- partial credentials: return the existing auth error
- full credentials: construct the classic private adapter directly

The adapter should no longer:

- call account settings to detect account mode
- validate UTA compatibility
- infer whether the account is classic or unified

If a user supplies a non-classic Bitget account, private calls may fail at runtime. That is acceptable under this design because the adapter no longer claims multi-account support.

## Files To Delete

- `bitget/private_uta.go`
- `bitget/sdk/private_ws_trade_uta.go`

## Files To Simplify

- `bitget/options.go`
  - remove `AccountMode`
  - remove `accountMode()` parsing helper
- `bitget/common.go`
  - remove all account-mode constants and detection helpers
  - keep only shared parsing, symbol, interval, and auth helpers still used by classic/public code
- `bitget/private_profile.go`
  - remove mode constants and mode-switch constructors
- `bitget/perp_adapter.go`
  - remove `accountMode` field and mode-dependent initialization
- `bitget/spot_adapter.go`
  - remove `accountMode` field and mode-dependent initialization
- `bitget/private_init_test.go`
  - rewrite to assert classic-only constructor semantics
- `bitget/ws_order_mode_test.go`
  - remove UTA routing tests
  - retain classic routing tests and default-REST regression coverage
- `bitget/adapter_test.go`
  - keep live tests, but remove any wording or assumptions implying UTA support
- Bitget docs/spec artifacts in this worktree
  - update or supersede text that claims UTA support

## Required Behavioral Outcomes

After the change:

- public Bitget construction still works with empty credentials
- private Bitget construction still works with full credentials
- partial credentials still fail fast
- classic spot and classic perp live suites remain the supported target
- classic WS order mode remains available when explicitly enabled
- no code path should reference `utaPerpProfile`, `utaSpotProfile`, or UTA WS trade helpers

## Test Strategy

The implementation should keep or add tests for:

- public-only initialization still works
- partial credentials still fail
- constructors no longer expose or depend on account mode
- classic WS order routing still works in `OrderModeWS`
- default Bitget constructor order mode remains REST

The implementation should remove or rewrite tests that assume:

- UTA account acceptance
- auto-detection between UTA and classic
- UTA WS routing

Live verification target after implementation:

- `go test ./bitget -run 'Test(Perp|Spot)Adapter_(Compliance|Orders|OrderQuerySemantics|Lifecycle|LocalState)' -count=1 -v`
- optional classic WS live tests behind `BITGET_ENABLE_WS_ORDER_TESTS=1`

## Merge Criteria

This change is complete when:

- all UTA-specific Bitget code is removed
- the package compiles without dead UTA references
- Bitget package tests pass
- live tests validate the classic-only path
- the supported behavior is simpler and accurately documented
