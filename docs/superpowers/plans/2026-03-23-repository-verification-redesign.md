# Repository Verification Redesign Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the repository's ambiguous `go test ./...` behavior with explicit quick, exchange-scoped, full-regression, and optional soak verification entrypoints.

**Architecture:** Introduce a small verification runtime around the existing Go tests instead of continuing to treat raw `go test ./...` as the source of truth. The implementation has three moving parts: shared test gating helpers in Go, shell entrypoints under `scripts/`, and targeted test updates so live or credentialed tests declare whether they belong to quick, full, or soak verification.

**Tech Stack:** Go tests, shell scripts, `.env` loading, internal test helper package, README and verification docs

---

## Planned File Map

- Create: `internal/testenv/testenv.go`
  Purpose: centralize repo-root `.env` loading, `RUN_FULL` / `RUN_SOAK` gating, and required-env skip helpers for tests.
- Create: `internal/testenv/testenv_test.go`
  Purpose: verify env-loading and gating behavior without relying on live exchange tests.
- Create: `scripts/verify_exchange.sh`
  Purpose: run exchange-scoped quick verification via explicit package mapping.
- Create: `scripts/verify_full.sh`
  Purpose: load `.env`, validate required variables, export `RUN_FULL=1`, run the canonical full regression commands.
- Modify: `.env.example`
  Purpose: align example variable names with the canonical full-regression env contract from the spec.
- Modify: `README.md`
  Purpose: replace the old testing guidance with quick/exchange/full/soak verification commands.
- Modify: `docs/superpowers/plans/2026-03-23-repository-test-gate-triage.md`
  Purpose: link the triage note to the landed redesign and narrow it to residual true-bug follow-up.

Fast-gate test files that need `testing.Short()` or full/soak gating:
- Modify: `aster/sdk/perp/ws_market_test.go`
- Modify: `binance/sdk/perp/ws_market_test.go`
- Modify: `edgex/sdk/perp/account_test.go`
- Modify: `grvt/sdk/account_test.go`
- Modify: `grvt/sdk/ws_market_test.go`
- Modify: `grvt/sdk/ws_order_test.go`
- Modify: `hyperliquid/sdk/client_test.go`
- Modify: `hyperliquid/sdk/perp/ws_account_test.go`
- Modify: `lighter/sdk/common/nonce_test.go`
- Modify: `nado/sdk/account_test.go`
- Modify: `nado/sdk/market_test.go`
- Modify: `okx/sdk/ws_market_test.go`
- Modify: `okx/sdk/ws_order_test.go`
- Modify: `edgex/adapter_test.go`
- Modify: `nado/adapter_test.go`
- Modify: `okx/adapter_test.go`

Files that likely need small failure-mode fixes so full regression can run cleanly once tests are correctly gated:
- Modify: `lighter/sdk/order.go`
- Modify: `hyperliquid/sdk/client.go`

## Task 1: Add Shared Test-Environment Helpers

**Files:**
- Create: `internal/testenv/testenv.go`
- Create: `internal/testenv/testenv_test.go`

- [ ] **Step 1: Write the failing helper tests**

Cover these cases:

```go
func TestRequireFullSkipsWithoutRunFull(t *testing.T) {}
func TestRequireFullSkipsWhenRequiredEnvMissing(t *testing.T) {}
func TestRequireSoakSkipsWithoutRunSoak(t *testing.T) {}
func TestLoadRepoEnvDoesNotOverrideExistingEnv(t *testing.T) {}
```

- [ ] **Step 2: Run helper tests to verify they fail**

Run: `go test ./internal/testenv -v`
Expected: FAIL because the package does not exist yet.

- [ ] **Step 3: Implement the helper package**

`internal/testenv/testenv.go` should provide:

- a repo-root `.env` loader used by tests
- `RequireFull(t, vars...)`
- `RequireSoak(t, vars...)`
- `RequireEnv(t, vars...)`

Behavior:
- load `.env` from repo root if present
- never override already-exported shell env vars
- `RequireFull` skips unless `RUN_FULL=1`
- `RequireSoak` skips unless `RUN_SOAK=1`
- env-missing cases skip with actionable messages

- [ ] **Step 4: Run helper tests to verify they pass**

Run: `go test ./internal/testenv -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/testenv/testenv.go internal/testenv/testenv_test.go
git commit -m "test: add shared verification env helpers"
```

## Task 2: Create Canonical Verification Scripts

**Files:**
- Create: `scripts/verify_exchange.sh`
- Create: `scripts/verify_full.sh`
- Modify: `.env.example`

- [ ] **Step 1: Write the failing script smoke checks**

Add narrow checks in a temporary script test harness or shell-based assertions if the repo already has no shell-test framework. The minimum behaviors to cover are:

- `verify_exchange.sh unknown` fails
- `verify_full.sh` fails clearly when `.env` is missing
- `verify_full.sh` fails clearly when required env vars are missing

- [ ] **Step 2: Run the smoke checks to verify they fail**

Run the chosen shell-test command(s).
Expected: FAIL because the scripts do not exist yet.

- [ ] **Step 3: Implement `scripts/verify_exchange.sh`**

Requirements:
- `#!/usr/bin/env bash`
- `set -euo pipefail`
- map each exchange name to one explicit package glob
- fail fast on unknown exchange
- run `go test -short <resolved-package-glob>`

The mapping must be exactly:
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

- [ ] **Step 4: Implement `scripts/verify_full.sh`**

Requirements:
- load `.env` from repo root
- exported shell env wins over `.env`
- fail immediately if `.env` is missing
- fail immediately if any required variables are missing
- export `RUN_FULL=1`
- run:
  - `go test -short ./...`
  - `go test ./aster/sdk/perp ./binance/sdk/perp ./edgex/sdk/perp ./grvt/sdk ./hyperliquid/sdk ./hyperliquid/sdk/perp ./lighter/sdk ./lighter/sdk/common ./nado/sdk ./okx/sdk ./standx/sdk`

Required env vars:
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

- [ ] **Step 5: Align `.env.example` with the canonical env contract**

Update mismatched names so the example file matches the full-regression script and tests. In particular:
- `OKX_SECRET_KEY` -> `OKX_API_SECRET`
- `EDGEX_PRIVATE_KEY` -> `EDGEX_STARK_PRIVATE_KEY`
- `NADO_SUB_ACCOUNT_NAME` -> `NADO_SUBACCOUNT_NAME`

- [ ] **Step 6: Run the script smoke checks to verify they pass**

Run the chosen shell-test command(s) again.
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add scripts/verify_exchange.sh scripts/verify_full.sh .env.example
git commit -m "build: add canonical verification scripts"
```

## Task 3: Gate Credential-Dependent Tests Behind Full Verification

**Files:**
- Modify: `edgex/sdk/perp/account_test.go`
- Modify: `grvt/sdk/account_test.go`
- Modify: `hyperliquid/sdk/client_test.go`
- Modify: `nado/sdk/account_test.go`
- Modify: `nado/sdk/market_test.go`
- Modify: `okx/sdk/ws_market_test.go`
- Modify: `okx/sdk/ws_order_test.go`
- Modify: `standx/sdk/account_test.go`
- Modify: `standx/sdk/order_test.go`
- Modify: `edgex/adapter_test.go`
- Modify: `nado/adapter_test.go`
- Modify: `okx/adapter_test.go`

- [ ] **Step 1: Add failing gating assertions where no full gate exists yet**

Introduce at least one focused assertion per package that proves a full-only test now skips outside `RUN_FULL=1`.

- [ ] **Step 2: Run the targeted package tests to verify current failures**

Run targeted commands without `RUN_FULL`:
- `go test ./edgex ./edgex/sdk/perp ./grvt/sdk ./hyperliquid/sdk ./nado ./nado/sdk ./okx ./okx/sdk ./standx/sdk -count=1 -v`

Expected: at least some tests fail or attempt live access because full gating is missing.

- [ ] **Step 3: Update the tests to use `internal/testenv`**

For each full-only test:
- keep `testing.Short()` behavior if already present and still appropriate
- add `testenv.RequireFull(t, <vars...>)`
- remove ad-hoc `.env` loading where the helper now centralizes it
- align adapter tests to the canonical env names:
  - `EDGEX_STARK_PRIVATE_KEY`
  - `NADO_SUBACCOUNT_NAME`
  - `OKX_API_SECRET`

- [ ] **Step 4: Run the targeted package tests outside full mode**

Run the same command as Step 2 without `RUN_FULL`.
Expected: PASS with skips instead of live failures.

- [ ] **Step 5: Run the targeted package tests in full mode**

Run with `RUN_FULL=1` and your local `.env`.
Expected: tests execute instead of skipping; any remaining failures are now true package issues rather than gate issues.

- [ ] **Step 6: Commit**

```bash
git add edgex/sdk/perp/account_test.go grvt/sdk/account_test.go hyperliquid/sdk/client_test.go nado/sdk/account_test.go nado/sdk/market_test.go okx/sdk/ws_market_test.go okx/sdk/ws_order_test.go standx/sdk/account_test.go standx/sdk/order_test.go edgex/adapter_test.go nado/adapter_test.go okx/adapter_test.go
git commit -m "test: gate credential verification behind full mode"
```

## Task 4: Move Long-Running Websocket Coverage Out Of Quick Verification

**Files:**
- Modify: `aster/sdk/perp/ws_market_test.go`
- Modify: `binance/sdk/perp/ws_market_test.go`
- Modify: `grvt/sdk/ws_market_test.go`
- Modify: `grvt/sdk/ws_order_test.go`
- Modify: `hyperliquid/sdk/perp/ws_account_test.go`
- Modify: `lighter/sdk/common/nonce_test.go`
- Modify: `okx/sdk/ws_market_test.go`
- Modify: `okx/sdk/ws_order_test.go`

- [ ] **Step 1: Write or update focused gating tests**

At minimum, add one narrow assertion or table-driven check per package where feasible that the long-run tests skip unless their intended mode is active.

- [ ] **Step 2: Run targeted commands in short mode to show current exposure**

Run:
- `go test -short ./aster/sdk/perp ./binance/sdk/perp ./grvt/sdk ./hyperliquid/sdk/perp ./lighter/sdk/common ./okx/sdk -count=1 -v`

Expected: current live/long-run tests still run or fail because they are not correctly gated.

- [ ] **Step 3: Reclassify tests**

Rules to apply:
- live websocket observation tests that are still useful for full regression must require `RUN_FULL=1` and have bounded durations
- tests that are still long-run observation by nature must require `RUN_SOAK=1`
- no test in this set may wait 30 minutes

Specific guidance:
- shrink `hyperliquid/sdk/perp/ws_account_test.go` waits from 30 minutes to a bounded short interval only if the tests remain in Full Regression; otherwise mark them soak-only
- add `testing.Short()` skips for anything that must never run in Quick Verify
- ensure `lighter/sdk/common/nonce_test.go` is excluded from Quick Verify because it depends on a real external nonce service
- classify `okx/sdk/ws_market_test.go` and `okx/sdk/ws_order_test.go` explicitly:
  - bounded credentialed WS flows stay in Full Regression with `RequireFull`
  - purely observational waits move to Soak and must not stay on the default path

- [ ] **Step 4: Run short-mode targeted verification**

Run the Step 2 command again.
Expected: PASS

- [ ] **Step 5: Run full-mode targeted verification**

Run the affected packages with `RUN_FULL=1` for the tests intended to remain in Full Regression.
Expected: bounded execution without long hangs.

- [ ] **Step 6: Commit**

```bash
git add aster/sdk/perp/ws_market_test.go binance/sdk/perp/ws_market_test.go grvt/sdk/ws_market_test.go grvt/sdk/ws_order_test.go hyperliquid/sdk/perp/ws_account_test.go lighter/sdk/common/nonce_test.go okx/sdk/ws_market_test.go okx/sdk/ws_order_test.go
git commit -m "test: separate quick and soak websocket coverage"
```

## Task 5: Add The Soak Entrypoint After Classification

**Files:**
- Create: `scripts/verify_soak.sh`

- [ ] **Step 1: Write the failing soak-entrypoint smoke check**

Minimum behavior to cover:
- `verify_soak.sh` refuses to run unless `RUN_SOAK=1`
- `verify_soak.sh` only executes the explicit soak targets selected in Task 4

- [ ] **Step 2: Run the smoke check to verify it fails**

Run the chosen shell-test command(s).
Expected: FAIL because the script does not exist yet.

- [ ] **Step 3: Implement `scripts/verify_soak.sh`**

Requirements:
- `#!/usr/bin/env bash`
- `set -euo pipefail`
- require `RUN_SOAK=1`
- run only the explicit soak targets selected in Task 4
- do not run the full repo

- [ ] **Step 4: Run the smoke check to verify it passes**

Run the chosen shell-test command(s) again.
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add scripts/verify_soak.sh
git commit -m "build: add soak verification entrypoint"
```

## Task 6: Fix Failure Modes Exposed By The New Gate

**Files:**
- Modify: `lighter/sdk/order.go`
- Modify: `hyperliquid/sdk/client.go`
- Add or Modify: `lighter/sdk/order_test.go`
- Add or Modify: `hyperliquid/sdk/client_test.go`

- [ ] **Step 1: Write the failing regression tests**

Add focused tests for:
- `lighter` returning a normal error instead of panicking when credentials are invalid or signer state is unavailable
- `hyperliquid` handling the current user-fee payload shape correctly

- [ ] **Step 2: Run the focused regression tests to verify they fail**

Run package-scoped test commands for only the new regressions.
Expected: FAIL

- [ ] **Step 3: Implement the minimal fixes**

Requirements:
- `lighter/sdk/order.go` must not dereference a missing `KeyManager`
- `hyperliquid/sdk/client.go` must match the actual `GetUserFees` payload shape observed in your local full regression

- [ ] **Step 4: Run the focused regression tests to verify they pass**

Run the same focused commands.
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add lighter/sdk/order.go lighter/sdk/order_test.go hyperliquid/sdk/client.go hyperliquid/sdk/client_test.go
git commit -m "fix: stabilize full verification package failures"
```

## Task 7: Update Documentation And Developer Entry Points

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/plans/2026-03-23-repository-test-gate-triage.md`

- [ ] **Step 1: Update README verification guidance**

Replace the old testing section with:
- `go test -short ./...` for quick verify
- `scripts/verify_exchange.sh <exchange>` for exchange-scoped verify
- `scripts/verify_full.sh` for full regression
- `RUN_SOAK=1 scripts/verify_soak.sh` for long-run soak

- [ ] **Step 2: Update the triage note**

Mark the gate redesign as landed and reduce the note to residual follow-up items only.

- [ ] **Step 3: Verify docs examples against actual scripts**

Run:
- `bash scripts/verify_exchange.sh backpack` or another low-risk package target
- `bash scripts/verify_full.sh` only when `.env` is ready

Expected: command names in docs match the actual files.

- [ ] **Step 4: Commit**

```bash
git add README.md docs/superpowers/plans/2026-03-23-repository-test-gate-triage.md
git commit -m "docs: publish repository verification commands"
```

## Task 8: Final Verification

**Files:**
- Modify only if verification exposes a real issue in the files above

- [ ] **Step 1: Run quick verification**

Run: `go test -short ./...`
Expected: PASS

- [ ] **Step 2: Run exchange-scoped verification on at least one touched exchange**

Run:
- `bash scripts/verify_exchange.sh okx`
- `bash scripts/verify_exchange.sh hyperliquid`

Expected: PASS

- [ ] **Step 3: Run full regression**

Run: `bash scripts/verify_full.sh`
Expected: PASS within the bounded 600 second target on the current machine with `.env` configured.

- [ ] **Step 4: Verify the soak entrypoint behaves correctly**

Run without env:
- `bash scripts/verify_soak.sh`
Expected: FAIL or exit clearly because `RUN_SOAK=1` is required.

Run with env:
- `RUN_SOAK=1 bash scripts/verify_soak.sh`
Expected: only the selected soak tests run.

- [ ] **Step 5: Commit any final verification sync**

```bash
git add README.md .env.example scripts internal/testenv aster/sdk/perp/ws_market_test.go binance/sdk/perp/ws_market_test.go edgex/adapter_test.go edgex/sdk/perp/account_test.go grvt/sdk/account_test.go grvt/sdk/ws_market_test.go grvt/sdk/ws_order_test.go hyperliquid/sdk/client.go hyperliquid/sdk/client_test.go hyperliquid/sdk/perp/ws_account_test.go lighter/sdk/common/nonce_test.go lighter/sdk/order.go lighter/sdk/order_test.go nado/adapter_test.go nado/sdk/account_test.go nado/sdk/market_test.go okx/adapter_test.go okx/sdk/ws_market_test.go okx/sdk/ws_order_test.go standx/sdk/account_test.go standx/sdk/order_test.go docs/superpowers/plans/2026-03-23-repository-test-gate-triage.md
git commit -m "test: land repository verification redesign"
```
