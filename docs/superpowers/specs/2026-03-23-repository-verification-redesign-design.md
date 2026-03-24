# Repository Verification Redesign

## Goal

Replace the repository's ambiguous test/verification behavior with a layered verification model that is explicit, stable, and usable on a normal developer machine.

This redesign does not treat the current `go test ./...` behavior as the source of truth. The current default path mixes unit tests, live websocket tests, credential-dependent integration tests, and long-running soak tests. That makes the repository gate noisy, slow, and hard to interpret.

The new design defines two official verification entrypoints plus one optional soak entrypoint:

- quick verify for fast local validation
- full regression for complete local validation with `.env`
- soak for long-running websocket observation

## Problems With The Current State

The repository currently has four overlapping problems:

1. `go test ./...` mixes fundamentally different test categories.
2. many live tests have no explicit environment or duration gate
3. some tests require credentials but do not skip when credentials are absent
4. a few true SDK failures are hidden inside the broader test noise

This means the repository does not currently have a reliable answer to:

- what is the default local gate?
- what is the full local acceptance path?
- what is an optional long-running live soak path?

## Verification Layers

The repository should use two official verification layers plus one optional soak layer.

### 1. Quick Verify

Purpose:
- default local development gate
- the standard verification step right after adding a new SDK or adapter

Properties:
- must be stable without private credentials
- must avoid long-lived realtime waiting
- must finish in 600 seconds or less, measured by wall-clock elapsed time on the developer machine running the command

Contents:
- unit tests
- deterministic SDK tests
- deterministic adapter tests
- short integration tests that do not require live account credentials and have bounded time

Command target:
- `go test -short ./...`

This becomes the repository's primary fast gate.

### 2. Full Regression

Purpose:
- complete local regression path for a developer who has `.env` configured
- the standard verification step after a broad or risky repository change

Properties:
- may use real credentials and real exchange connectivity
- must still remain bounded and intentionally designed
- must finish in 600 seconds or less, measured by wall-clock elapsed time on the developer machine running the command

Contents:
- everything in Quick Gate
- real-account tests that can complete in a short, bounded window
- websocket tests that can actively drive the event they need rather than waiting indefinitely

Command target:
- `scripts/verify_full.sh`

This is the canonical "I changed a lot and want the full repository regression pass" path.

### 3. Soak / Live Long-Run

Purpose:
- optional long-running websocket observation
- reconnect stability checks
- event-stream quality checks

Properties:
- never part of the default local or CI gate
- must require explicit opt-in
- may run for multiple minutes or longer

Contents:
- long subscription tests
- long reconnect loops
- long event-flow observation tests

Command target:
- `RUN_SOAK=1 scripts/verify_soak.sh`

## Test Classification Rules

Every nontrivial test should clearly belong to one of these groups:

### Unit

- no network
- no credentials
- deterministic
- always part of Quick Gate

### Fast Integration

- may touch real network
- may require credentials
- must have explicit skip rules when credentials or prerequisites are missing
- must have bounded duration
- eligible for Full Regression
- eligible for Quick Verify only if they do not depend on private credentials and are stable under `-short`

### Soak / Long-Run Live

- long-lived websocket subscriptions
- event waiting that depends on natural market behavior
- reconnect or stream-observation runs longer than normal integration tests
- explicit opt-in only

## Classification Mechanism

Test membership is declared by mechanism, not by convention alone:

- Quick Gate membership:
  - the test runs under `go test -short ./...`
  - it must not require private credentials
  - any live network dependency must still be bounded and deterministic
- Full Validation membership:
  - the test skips under `testing.Short()`
  - it may require `.env` or real exchange connectivity
  - it must check prerequisites first and `t.Skip` when required env vars are absent
  - it must complete within the bounded full-validation budget
- Soak membership:
  - the test checks an explicit opt-in env flag such as `RUN_SOAK=1`
  - if that flag is absent, the test must `t.Skip`
  - it must not run in either Quick Gate or Full Validation by default

This makes the classification enforceable because each layer maps to a concrete command path and skip mechanism.

## Official Verification Commands

The repository should expose these official commands:

### Quick Verify

- `go test -short ./...`

This is the canonical fast validation command.

### Exchange Verify

- `scripts/verify_exchange.sh <exchange>`

This is the canonical fast command immediately after adding or changing one exchange package.

It should:

1. resolve `<exchange>` to one explicit package glob
2. fail fast when `<exchange>` is unknown
3. run `go test -short <resolved-package-glob>`

The package mapping is explicit and normative:

- `backpack` -> `./backpack/...`
- `aster` -> `./aster/...`
- `binance` -> `./binance/...`
- `bitget` -> `./bitget/...`
- `edgex` -> `./edgex/...`
- `grvt` -> `./grvt/...`
- `hyperliquid` -> `./hyperliquid/...`
- `lighter` -> `./lighter/...`
- `nado` -> `./nado/...`
- `okx` -> `./okx/...`
- `standx` -> `./standx/...`

### Full Regression

- `scripts/verify_full.sh`

This script must:

1. load `.env` from the repository root
2. allow already-exported shell variables to override values from `.env`
3. fail immediately if `.env` is missing
4. fail immediately if any required Full Regression variables are missing
5. export `RUN_FULL=1`
6. run:
   - `go test -short ./...`
   - `go test ./aster/sdk/perp ./binance/sdk/perp ./edgex/sdk/perp ./grvt/sdk ./hyperliquid/sdk ./hyperliquid/sdk/perp ./lighter/sdk ./lighter/sdk/common ./nado/sdk ./okx/sdk ./standx/sdk`

The required Full Regression variables are:

- `EDGEX_STARK_PRIVATE_KEY`
- `EDGEX_ACCOUNT_ID`
- `GRVT_API_KEY`
- `GRVT_SUB_ACCOUNT_ID`
- `GRVT_PRIVATE_KEY`
- `HYPERLIQUID_PRIVATE_KEY`
- `HYPERLIQUID_VAULT`
- `HYPERLIQUID_ACCOUNT_ADDR`
- `LIGHTER_PRIVATE_KEY`
- `LIGHTER_ACCOUNT_INDEX`
- `LIGHTER_KEY_INDEX`
- `NADO_PRIVATE_KEY`
- `NADO_SUBACCOUNT_NAME`
- `OKX_API_KEY`
- `OKX_API_SECRET`
- `OKX_API_PASSPHRASE`
- `STANDX_PRIVATE_KEY`

### Soak Validation

- `RUN_SOAK=1 scripts/verify_soak.sh`

This script must run only soak tests and must never be part of either Quick Verify or Full Regression.

## Required Test Behavior

The repository should enforce these rules across exchange SDK and adapter tests:

1. Any test that requires private credentials must check its prerequisites first and `t.Skip` when they are absent.
2. Any test that talks to a live exchange must have an explicit timeout.
3. No default-path test may wait for 30 minutes or rely on indefinite natural market activity.
4. Tests that need an order or account event should prefer actively triggering the event rather than passively waiting for the market.
5. Long-running websocket observation tests must be gated behind explicit opt-in environment variables.
6. `testing.Short()` should become the repository-standard mechanism for excluding non-quick tests from the default fast gate.

### Plain `go test ./...`

Plain `go test ./...` is no longer the canonical repository verification command after this redesign.

It may still be useful as an exploratory developer command, but merge/readiness is evaluated against:

- `go test -short ./...` for Quick Verify
- `scripts/verify_full.sh` for Full Regression
- `RUN_SOAK=1 scripts/verify_soak.sh` for Soak Validation

This removes the current ambiguity instead of trying to preserve it.

## Migration Strategy

The redesign should be implemented in phases.

### Phase 1: Establish The New Gate

- define the verification model in docs
- make `go test -short ./...` the supported quick gate
- add `scripts/verify_exchange.sh`
- add `scripts/verify_full.sh`
- add missing skip behavior for private-credential tests
- gate obvious long-running live tests so they do not run in Quick Gate

### Phase 2: Normalize Full Validation

- shorten or redesign realtime tests so they can fit within a bounded full-validation run
- prefer actively driven event tests over passive waiting
- document a single full-regression command sequence

### Phase 3: Isolate Soak Coverage

- move long-lived websocket observation tests behind explicit env flags
- document them as optional soak coverage rather than default acceptance coverage

## Immediate Application To Current Failures

The current repository failures should be interpreted under this redesign as follows:

- `aster/sdk/perp`, `binance/sdk/perp`, `grvt/sdk`, `okx/sdk`, and `hyperliquid/sdk/perp` websocket long-wait tests belong in Full Validation or Soak, not Quick Gate in their current form.
- `edgex`, `nado`, `grvt`, and `hyperliquid` credential-dependent tests need explicit prerequisite checks and should skip cleanly without required env vars.
- `lighter/sdk` nil-panic behavior and `hyperliquid/sdk` user-fee deserialization failure remain true code/debugging concerns after the gate is cleaned up.

## Success Criteria

This redesign is successful when:

1. `go test -short ./...` is a reliable repository quick gate.
2. credential-dependent tests skip cleanly when prerequisites are missing.
3. `scripts/verify_exchange.sh <exchange>` provides a fast exchange-scoped validation path after adding or editing one exchange package.
4. `scripts/verify_full.sh` is the canonical full local regression entrypoint, loads `.env` from the repository root, fails fast when required variables are missing, and completes within 600 seconds.
5. long-running websocket observation is opt-in rather than mixed into default validation.
6. remaining failures are easier to interpret because environment noise and soak noise have been removed from the default path.
