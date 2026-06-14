# Nautilus Core Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the missing NautilusTrader-equivalent core behavior and prove it through reusable Go parity testers.

**Architecture:** Treat `testsuite` as the product contract, `account` as the lifecycle source of truth, `platform` as live orchestration, `backtest` as a simulated venue, and `portfolio`/`risk` as account-facing accounting engines.

**Tech Stack:** Go 1.26, `shopspring/decimal`, repository-local SDK adapters, standard `go test` and race tests.

---

### Task 1: Nautilus-Style Contract Reports And Tester Entrypoints

**Files:**
- Create: `testsuite/report.go`
- Create: `testsuite/data_tester.go`
- Create: `testsuite/exec_tester.go`
- Modify: `testsuite/contracts_test.go`

- [x] Write failing tests requiring TC-D/TC-E style case IDs, names, statuses, and pass/fail reports.
- [x] Implement shared `ContractReport`, `CaseResult`, `DataTester`, and `ExecTester`.
- [x] Run `go test ./testsuite -run 'Test(DataTester|ExecTester)Reports' -count=1`.

### Task 2: Lifecycle Tester And Full Order Event Vocabulary

**Files:**
- Modify: `model/order.go`
- Create: `model/order_event.go`
- Create: `account/state_machine.go`
- Create: `testsuite/lifecycle_tester.go`
- Test: `model/order_event_test.go`
- Test: `account/state_machine_test.go`
- Test: `testsuite/lifecycle_tester_test.go`

- [x] Write failing tests for Nautilus order events: denied, emulated, released, submitted, accepted, rejected, triggered, pending-update, pending-cancel, updated, modify-rejected, cancel-rejected, canceled, expired, partially-filled, filled.
- [x] Implement event vocabulary and legal transition table.
- [x] Wire reconciler to reject illegal terminal/backward transitions while preserving venue report data.
- [x] Run `go test ./model ./account ./testsuite -run 'OrderEvent|StateMachine|Lifecycle' -count=1`.

### Task 3: Backtest Venue Semantics

**Files:**
- Modify: `backtest/runner.go`
- Create: `backtest/venue.go`
- Create: `backtest/matching.go`
- Test: `backtest/backtest_test.go`
- Test: `backtest/matching_test.go`

- [x] Write failing tests for existing-order matching before strategy callbacks.
- [x] Add regression coverage for same-timestamp cascading command settlement.
- [x] Write failing tests proving historical book data is not mutated.
- [x] Write failing tests for market-to-limit, stop-market, stop-limit, and trailing-stop-market behavior.
- [x] Add trailing-stop-limit coverage.
- [x] Implement matching and settlement for current advanced-order tests.
- [x] Run `go test -race ./backtest -count=1`.

### Task 4: Portfolio And Risk Parity

**Files:**
- Modify: `model/account.go`
- Modify: `model/order.go`
- Modify: `portfolio/portfolio.go`
- Modify: `risk/risk.go`
- Test: `portfolio/portfolio_test.go`
- Test: `risk/risk_test.go`
- Create: `testsuite/portfolio_tester.go`

- [x] Write failing tests for realized PnL, unrealized PnL, partial close, flip, and commission by currency.
- [x] Write failing tests for reduce-only safety, exposure limits, TIF, market notional, and instrument precision.
- [x] Implement minimal accounting and risk logic required by current tests.
- [x] Run `go test ./portfolio ./risk ./testsuite -count=1`.

### Task 5: Adapter Parity Rollout

**Files:**
- Modify: `testsuite/contracts.go`
- Modify: `adapter/*/*_test.go`
- Modify: `config/all/all_test.go`

- [x] Require venue contract data paths to pass `DataTester`, with unsupported stream cases reported as skipped unless required by capability tests.
- [x] Require venue contract execution paths to pass `ExecTester`, with unsupported private resubscribe reported as skipped unless required by capability tests.
- [x] Mark unsupported venue capabilities honestly instead of accepting no-op success.
- [x] Run `go test ./adapter/... ./config/all -count=1`.

### Task 6: Order List And Bracket Lifecycle Semantics

**Files:**
- Modify: `model/order.go`
- Modify: `strategy/engine.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `model/order_factory_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`

- [x] Add order-list metadata to normalized order reports so OTO/OCO relationships survive submission, reconciliation, and cache updates.
- [x] Add `SubmitOrderList` to the strategy runtime surface.
- [x] Implement backtest order-list handling where OTO children are held, released after parent fill, and OCO siblings are canceled after one child fills.
- [x] Implement the same delayed-child and OCO-cancel behavior in `platform.Node` for live private-stream/order-report progress.
- [x] Run `go test ./backtest -run TestBacktestBracketReleasesChildrenAndCancelsOcoSibling -count=1`.
- [x] Run `go test ./platform -run TestNodeSubmitOrderListHoldsChildrenUntilParentFilledAndCancelsOcoSibling -count=1`.
- [x] Run `go test -count=1 ./model ./strategy ./platform ./backtest ./account ./cache ./risk ./portfolio ./testsuite`.

### Task 7: Full Verification

- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -count=1 ./adapter/... ./config/all`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.

### Task 8: Position Lifecycle Parity

**Files:**
- Create: `model/position_event.go`
- Modify: `model/account.go`
- Modify: `account/reconciler.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Modify: `strategy/typed.go`
- Modify: `testsuite/lifecycle_tester.go`
- Test: `model/position_event_test.go`
- Test: `strategy/typed_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`
- Test: `testsuite/lifecycle_tester_test.go`

- [x] Add normalized `PositionLifecycleEvent` with opened/changed/closed classification from cached previous state.
- [x] Publish derived position lifecycle events from live `platform.Node` and deterministic `backtest` runtime after position reports are applied.
- [x] Dispatch generic and specific typed strategy callbacks: `OnPositionLifecycle`, `OnPositionOpened`, `OnPositionChanged`, and `OnPositionClosed`.
- [x] Add lifecycle tester coverage for position lifecycle classification.
- [x] Run `go test ./model ./strategy ./platform -run 'PositionLifecycle|TestNodePublishesDerivedPositionLifecycleEvents' -count=1`.
- [x] Run `go test ./backtest -run TestBacktestDispatchesPositionLifecycleCallbacks -count=1`.
- [x] Run `go test ./testsuite -run TestLifecycleTesterReportsNautilusOrderLifecycleCases -count=1`.

### Task 9: Final Verification After Position Lifecycle

- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 10: Risk Exposure Limits

**Files:**
- Modify: `risk/risk.go`
- Test: `risk/risk_test.go`
- Modify: `testsuite/risk_tester.go`
- Test: `testsuite/risk_tester_test.go`

- [x] Add projected per-position notional limit using existing cached position plus submitted order quantity.
- [x] Add projected account exposure limit across cached positions plus the submitted order.
- [x] Add RiskTester contract cases for position-notional and account-exposure rejection.
- [x] Run `go test ./risk -count=1`.
- [x] Run `go test ./testsuite -run TestRiskTesterReportsOrderSafetyCases -count=1`.

### Task 11: Final Verification After Risk Exposure Limits

- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 12: Risk Denial Lifecycle Events

**Files:**
- Modify: `model/order_event.go`
- Test: `model/order_event_test.go`
- Modify: `platform/node.go`
- Test: `platform/node_test.go`

- [x] Allow denied order lifecycle events to carry client order ID without venue/order ID, matching submit-time risk denial semantics.
- [x] Publish `OrderDenied` lifecycle events when platform risk checks reject a submit or parent order-list submit.
- [x] Preserve `SubmitOrder` error return while also notifying strategy/runtime subscribers.
- [x] Run `go test ./model ./platform -run 'RiskDenied|OrderDeniedWhenRiskRejectsSubmit' -count=1`.

### Task 13: Final Verification After Risk Denial Lifecycle

- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 14: Trading State And Reduce-Only Flip Safety

**Files:**
- Modify: `risk/risk.go`
- Test: `risk/risk_test.go`
- Modify: `testsuite/risk_tester.go`
- Test: `testsuite/risk_tester_test.go`

- [x] Add `TradingStateActive`, `TradingStateHalted`, and `TradingStateReducing` pre-trade checks.
- [x] Reject reduce-only or reducing-state orders that would flip into opposite-side exposure.
- [x] Add RiskTester coverage for reduce-only flip rejection.
- [x] Run `go test ./risk -count=1`.
- [x] Run `go test ./testsuite -run TestRiskTesterReportsOrderSafetyCases -count=1`.

### Task 15: Final Verification After Trading State

- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 16: Modify Order Lifecycle Closure

**Nautilus reference:**
- `ModifyOrder` command carries client/venue identity plus optional quantity, price, and trigger price.
- `OrderPendingUpdate` maps to `Accepted -> PendingUpdate`.
- `OrderUpdated` and `OrderModifyRejected` map to `PendingUpdate -> Accepted`.
- Execution clients expose a `modify_order` command hook and adapters either perform venue cancel/replace/update or emit `OrderModifyRejected`.

**Files:**
- Modify: `model/order.go`
- Modify: `strategy/engine.go`
- Modify: `venue/interfaces.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `model/model_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`
- Test maintenance: `strategy/strategy_test.go`, `strategy/typed_test.go`, `platform/node_test.go`

- [x] Add `model.ModifyOrder` with identity and changed-field validation.
- [x] Add reusable order modification application with order-type, terminal-state, and filled-quantity guards.
- [x] Add `Runtime.ModifyOrder` so strategies can issue Nautilus-style modify commands.
- [x] Add optional `venue.OrderModifier` for execution clients without forcing every adapter to claim support.
- [x] Implement `platform.Node.ModifyOrder` with pending-update lifecycle, risk/precision checks, venue modification, normalized updated reports, and modify-rejected restoration.
- [x] Implement `backtest` modify handling with deterministic pending-update/updated lifecycle dispatch and immediate rematching against current simulated venue data.
- [x] Fix async private fill/position test waiting so race verification observes both forwarded reports.
- [x] Run `go test ./model -run TestModifyOrderRequiresIdentityAndAChangedField -count=1`.
- [x] Run `go test ./platform -run 'TestNodeModifyOrder' -count=1`.
- [x] Run `go test ./backtest -run TestBacktestModifyOrderUpdatesRestingOrderAndMatches -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -count=1 ./adapter/... ./config/all`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `git diff --check`.
- [x] Note: an earlier full non-SDK run exposed one OKX subscribe/unsubscribe timeout, then `go test -count=1 ./adapter/okx -run TestSpotClientsPassVenueContractSuite` passed on rerun; the final full non-SDK verification also passed.

### Task 17: Cancel Order Lifecycle Closure

**Nautilus reference:**
- `CancelOrder` creates an `OrderPendingCancel` event before routing to execution.
- Successful venue cancellation produces `OrderCanceled`.
- Venue rejection produces `OrderCancelRejected` and returns the order to `Accepted`.
- `OrderCanceled` may transition from `PendingCancel` or directly from `Accepted`; the platform path should prefer the explicit pending lifecycle when the command originates locally.

**Files:**
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`

- [x] Add platform cancel lifecycle: cached order lookup, `PendingCancel` report, `OrderPendingCancel`, venue cancel, normalized `Canceled` report, and `OrderCanceled`.
- [x] Add platform cancel rejection lifecycle: restore accepted state and publish `OrderCancelRejected` when venue cancel fails.
- [x] Add backtest cancel lifecycle with deterministic typed strategy callbacks for pending cancel and canceled.
- [x] Preserve existing OCO sibling cancellation behavior through the new lifecycle path.
- [x] Run `go test ./platform -run 'TestNodeCancelOrder' -count=1`.
- [x] Run `go test ./backtest -run TestBacktestCancelOrderDispatchesPendingCancelAndCanceled -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 18: Batch Cancel And Cancel-All Runtime Surface

**Nautilus reference:**
- `BatchCancelOrders` is a first-class trading command carrying an instrument and a non-empty list of `CancelOrder` commands.
- `CancelAllOrders` is a first-class command for clearing open orders on a venue/instrument.
- Venue-native batch cancel can optimize transport later, but platform semantics must remain per-order observable through normal cancel lifecycle events.

**Files:**
- Modify: `model/order.go`
- Modify: `strategy/engine.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `model/model_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`
- Test maintenance: `strategy/strategy_test.go`, `strategy/typed_test.go`

- [x] Add `model.BatchCancelOrders` and `model.CancelAllOrders` validation.
- [x] Add `Runtime.BatchCancelOrders` and `Runtime.CancelAllOrders`.
- [x] Implement platform batch cancel as a deterministic per-order fallback through `CancelOrder`, preserving pending/canceled/cancel-rejected lifecycle semantics.
- [x] Implement platform cancel-all from cached open orders filtered by account and instrument.
- [x] Implement matching backtest batch/cancel-all behavior.
- [x] Run `go test ./model -run TestBatchCancelAndCancelAllValidateCommandShape -count=1`.
- [x] Run `go test ./platform -run 'TestNode(BatchCancelOrders|CancelAllOrders)' -count=1`.
- [x] Run `go test ./backtest -run TestBacktestCancelAllOrdersCancelsOpenOrdersForInstrument -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 66: Cancel-All Order-Side Filtering Closure

**Nautilus reference:**
- `CancelAllOrders` can target an instrument and optionally a side, so strategies can cancel only buy or sell working orders without sweeping the whole book.
- Live and simulated runtimes must apply identical command semantics before routing individual cancels.

**Files:**
- Modify: `model/order.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `model/model_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`

- [x] Add optional `OrderSide` to `model.CancelAllOrders`.
- [x] Validate side values and preserve existing account/instrument command validation.
- [x] Add reusable `MatchesOrder` filtering so live and backtest runtimes share one side/instrument predicate.
- [x] Apply the side filter in `platform.Node.CancelAllOrders`.
- [x] Apply the side filter in `backtest.runtime.CancelAllOrders`.
- [x] Run `go test ./model -run TestBatchCancelAndCancelAllValidateCommandShape -count=1`.
- [x] Run `go test ./platform -run TestNodeCancelAllOrdersFiltersByOrderSide -count=1`.
- [x] Run `go test ./backtest -run TestBacktestCancelAllOrdersFiltersByOrderSide -count=1`.

### Task 67: Portfolio Concurrent Event Safety Closure

**Nautilus reference:**
- Portfolio and account state are shared runtime services fed by asynchronous market-data and execution events.
- Accounting reads and writes must be safe while live data, fills, and strategy reads are concurrent.

**Files:**
- Modify: `portfolio/portfolio.go`
- Test: `portfolio/portfolio_test.go`

- [x] Add internal locking around fill accounting, market marks, balance updates, positions, realized PnL, and commission maps.
- [x] Preserve atomic accounting semantics for partial closes, flips, mark-to-market PnL, and equity reads.
- [x] Add race coverage that concurrently applies fills, mark updates, equity reads, and PnL reads.
- [x] Run `go test -race ./portfolio -run TestPortfolioConcurrentFillAndMarketUpdates -count=1`.

### Task 68: Strategy Actor-Serial Dispatch Closure

**Nautilus reference:**
- Strategies behave like actors: callbacks are delivered serially for a strategy instance even when multiple topics are producing events.
- Public market data, private execution events, and timers should not reenter the same strategy concurrently.

**Files:**
- Modify: `strategy/engine.go`
- Test: `strategy/strategy_test.go`

- [x] Replace per-topic direct callback goroutines with a fan-in event channel and one async dispatcher loop.
- [x] Preserve multi-topic subscription fan-out through the shared message bus while serializing callback execution per engine.
- [x] Add a non-reentrant strategy regression test that fails if two subscribed topics call into the same strategy concurrently.
- [x] Run `go test ./strategy -run TestEngineSerializesAsyncCallbacksPerStrategy -count=1`.
- [x] Run `go test -race ./strategy ./portfolio -count=1`.

### Task 69: Nautilus Order State Table And Reconciliation Normalization

**Nautilus reference:**
- The order lifecycle is a transition table, not a generic forward-only enum.
- Local pending-update/pending-cancel states can receive venue echoes and late fills; reconciliation should normalize those reports instead of rejecting valid real-world sequences.

**Files:**
- Modify: `account/state_machine.go`
- Modify: `account/reconciler.go`
- Test: `account/state_machine_test.go`
- Test: `account/reconciler_test.go`

- [x] Replace flat allowed-transition validation with a Nautilus-style `from -> incoming -> next` state table.
- [x] Add `NextOrderStatus` so reconciliation can keep local pending states when a venue reports an accepted/submitted echo.
- [x] Allow late partial/filled reports while cancel is pending, and allow realistic terminal repair paths such as canceled to filled.
- [x] Normalize reconciled order status through the state table before cache mutation.
- [x] Run `go test ./account -run 'TestCanOrderTransitionFollowsNautilusLifecycle|TestNextOrderStatusKeepsPendingUpdateOnSubmittedEcho' -count=1`.
- [x] Run `go test ./account -run 'TestReconcilerAcceptsVenueFillsWhileCancelPending|TestReconcilerKeepsPendingUpdateOnSubmittedEcho' -count=1`.

### Task 70: Backtest Order Latency Closure

**Nautilus reference:**
- Backtests need deterministic exchange latency modeling so accepted orders are not always immediately matchable on the same tick.
- Latency must affect matching eligibility without mutating the command or market-data chronology.

**Files:**
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Add `Config.OrderLatency`.
- [x] Track per-order eligibility timestamps inside the simulated venue runtime.
- [x] Delay order matching until simulated time reaches the order eligibility timestamp.
- [x] Keep order acceptance lifecycle deterministic while deferring fills.
- [x] Run `go test ./backtest -run TestBacktestOrderLatencyDelaysOrderEligibility -count=1`.

### Task 71: Platform Default Portfolio Boundary Closure

**Nautilus reference:**
- Platform nodes should construct a coherent runtime graph by default: cache, portfolio, risk, data, execution, and strategy services share the same state boundary unless explicitly overridden.
- A user should not need manual portfolio wiring for the common live-node path.

**Files:**
- Modify: `platform/node.go`
- Test: `platform/node_test.go`

- [x] Create a default `portfolio.Portfolio` when `platform.Config.Portfolio` is nil.
- [x] Ensure the default portfolio shares the node cache rather than constructing a detached state store.
- [x] Preserve explicit portfolio injection for tests and advanced runtimes.
- [x] Run `go test ./platform -run TestNodeCreatesDefaultPortfolioWithSharedCache -count=1`.

### Task 72: Verification After Core Runtime Closures

- [x] Run `go test -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -race -count=1 ./account ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 73: Nautilus-Style Backtest FillModel Closure

**Nautilus reference:**
- `FillModel` exposes probabilistic limit-touch fills through `prob_fill_on_limit`.
- `FillModel` exposes L1 taker slippage through `prob_slippage`, moving fills one tick against the order direction when it fires.
- `random_seed` makes probabilistic simulations reproducible.
- L2/L3 order book fills still use depth/liquidity, while quote/trade/bar/ticker fills use the L1 slippage path.

**Files:**
- Create: `backtest/fill_model.go`
- Create: `backtest/fill_model_test.go`
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Add `FillModel`, `FillContext`, `FillSource`, `FillModelConfig`, and seeded `ProbabilisticFillModel`.
- [x] Preserve Nautilus defaults for `NewFillModel(FillModelConfig{})`: limit touch fills with probability `1.0`, slippage probability `0.0`.
- [x] Add explicit zero-probability handling for limit-touch fill rejection.
- [x] Apply `prob_fill_on_limit` before quote, trade, bar, and book limit-touch fills.
- [x] Apply one-tick `prob_slippage` to L1 taker fills using the instrument price tick and order side.
- [x] Keep order-book depth slippage governed by actual book levels.
- [x] Run `go test ./backtest -run 'Test(FillModel|BacktestFillModel)' -count=1`.
- [x] Run `go test ./backtest -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.

### Task 74: Portfolio Account Base Currency Conversion Closure

**Nautilus reference:**
- Portfolio queries can convert PnL, equity, mark values, and exposure to a target currency.
- For single-account queries without an explicit target, Nautilus converts to the account base currency when `convert_to_account_base_currency=true`.
- Conversion uses cached xrate prices and must not silently value missing exchange rates at `1.0`.

**Files:**
- Modify: `model/account.go`
- Modify: `portfolio/portfolio.go`
- Test: `portfolio/portfolio_test.go`
- Modify: `testsuite/portfolio_tester.go`
- Test: `testsuite/portfolio_tester_test.go`

- [x] Add `BaseCurrency` to `model.AccountSnapshot`.
- [x] Convert `Portfolio.Equity`, `AvailableEquity`, `MarkValues`, `RealizedPnLs`, `UnrealizedPnLs`, and `TotalPnLs` to account base currency when present.
- [x] Convert `Portfolio.Exposure(accountID, targetCurrency)` from native settlement/quote currency to the requested target currency.
- [x] Resolve exchange rates from cached quote ticks first, then ticker, trade tick, and latest bar fallback.
- [x] Preserve missing-rate safety by skipping values that cannot be converted instead of assuming a `1.0` rate.
- [x] Add `TC-P07 Account base currency conversion` to the shared portfolio contract tester.
- [x] Run `go test ./portfolio -run 'TestPortfolioConverts|TestPortfolioTracksAccountBalancesMarginsAndEquity' -count=1`.
- [x] Run `go test ./testsuite -run TestPortfolioTesterReportsPnLAndCommissionCases -count=1`.

### Task 75: Risk Account Exposure Base Currency Closure

**Nautilus reference:**
- Risk limits should be evaluated against portfolio/account valuation units, not by adding unrelated settlement currencies as if they were the same unit.
- Account-level exposure checks should use an explicit target currency or the account base currency when present.
- Missing exchange rates must block the exposure estimate instead of silently approving an order.

**Files:**
- Modify: `risk/risk.go`
- Test: `risk/risk_test.go`
- Modify: `testsuite/risk_tester.go`
- Test: `testsuite/risk_tester_test.go`

- [x] Add optional `risk.Config.ExposureCurrency`.
- [x] Use `ExposureCurrency` when set, otherwise use `AccountSnapshot.BaseCurrency` for `MaxAccountExposure`.
- [x] Convert projected account exposure from instrument settlement/quote currency through cached quote/ticker/trade/bar xrate prices.
- [x] Reject account exposure checks when a required conversion rate is missing.
- [x] Add `TC-R13 Base currency account exposure rejection` to the shared risk tester.
- [x] Run `go test ./risk -run 'TestEngineConvertsProjectedAccountExposureToAccountBaseCurrency|TestEngineRejectsOrdersExceedingProjectedAccountExposure|TestEngineUsesQuoteTickMarksForProjectedAccountExposure' -count=1`.
- [x] Run `go test ./risk -count=1`.
- [x] Run `go test ./testsuite -run TestRiskTesterReportsOrderSafetyCases -count=1`.

### Task 76: Verification After FillModel, Portfolio, And Risk Closures

- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -race -count=1 ./account ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 25: Bar OHLC Limit Matching Closure

**Nautilus reference:**
- Bar-driven backtests must use the full OHLC envelope for price-touch semantics, not only the close price.
- A resting buy limit is executable when the bar low touches or crosses the limit price; a resting sell limit is executable when the bar high touches or crosses the limit price.
- This task keeps the current conservative fill policy of filling at the order limit for OHLC-derived limit executions. A later full fill-model task can make optimistic/pessimistic/open/close execution policy configurable.

**Files:**
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Write failing coverage for buy limit orders touched by bar low while close remains above the limit.
- [x] Write failing coverage for sell limit orders touched by bar high while close remains below the limit.
- [x] Implement side-aware bar OHLC limit matching without changing market-order close matching behavior.
- [x] Run `go test ./backtest -run 'TestBacktestMatches(LimitOrderAgainstBarIntrabarLow|SellLimitOrderAgainstBarIntrabarHigh|MarketOrderAgainstBarClose)' -count=1`.

### Task 26: Bar OHLC Trigger Matching Closure

**Nautilus reference:**
- Stop and touched orders are matched by the execution matching core through explicit trigger checks such as `is_stop_triggered` and `is_touch_triggered`.
- A bar-based simulator must evaluate trigger conditions against the bar envelope, not only the close, otherwise intrabar stop/touch events disappear from backtests.
- This task keeps trigger-derived market fills conservative by recording the trigger price for bar-sourced stop-market and market-if-touched executions; a later configurable fill-model task can replace this with venue/model-specific slippage behavior.

**Files:**
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Write failing coverage for buy stop-market orders triggered by bar high while close remains below the trigger.
- [x] Write failing coverage for sell stop-market orders triggered by bar low while close remains above the trigger.
- [x] Add latest trigger-price candidate selection across ticker, quote, trade, and bar high/low snapshots.
- [x] Fill bar-sourced stop-market and market-if-touched executions at trigger price after the order is marked triggered.
- [x] Run `go test ./backtest -run 'TestBacktestTriggers(StopMarketOrderOnLaterTick|BuyStopMarketOrderOnBarIntrabarHigh|SellStopMarketOrderOnBarIntrabarLow)' -count=1`.

### Task 27: Bar OHLC Trailing Stop Closure

**Nautilus reference:**
- Trailing stops are matched in the execution matching core after activation, then behave like dynamic stop orders whose trigger price follows the favorable market extreme.
- A bar-based simulator must update the favorable extreme from the bar high/low and check the adverse side of the same bar for trigger penetration.
- This task records the dynamic trailing trigger price in the normalized order report before dispatching `OrderTriggered`; K-line sourced trailing-market fills use that dynamic trigger price.

**Files:**
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Write failing coverage for sell trailing-stop-market orders where one bar makes a new high and then trades through `high - trailing_offset`.
- [x] Track latest trailing ranges across ticker, quote, trade, and bar snapshots instead of a single close/last scalar.
- [x] Store dynamic trailing trigger prices and copy them onto triggered order reports.
- [x] Fill bar-sourced trailing-stop-market executions at the dynamic trigger price.
- [x] Run `go test ./backtest -run 'TestBacktestTriggersTrailingStop(MarketOnBarIntrabarHighLow|MarketFromActivatedHighWatermark|LimitFromActivatedHighWatermark)' -count=1`.

### Task 28: Risk Market Price Sources Closure

**Nautilus reference:**
- Risk checks must consume the same normalized market-data cache used by data, execution, portfolio, and strategy runtime.
- Market order notional and projected account exposure cannot depend only on ticker/order-book data once quote ticks, trade ticks, and bars are first-class data objects.
- Position marks should use side-aware quote prices when available, then fall back to other normalized market data.

**Files:**
- Modify: `cache/cache.go`
- Modify: `risk/risk.go`
- Modify: `testsuite/risk_tester.go`
- Test: `cache/cache_test.go`
- Test: `risk/risk_test.go`
- Test: `testsuite/risk_tester_test.go`

- [x] Add cache indexing for the latest bar by instrument.
- [x] Write failing coverage for market-order notional rejection using quote-only top-of-book data.
- [x] Write failing coverage for projected account exposure using quote tick marks for existing positions.
- [x] Use quote ticks, trade ticks, and latest bars as risk price sources for order notional and position marks.
- [x] Add `TC-R08 Quote tick market notional rejection` to the shared risk contract suite.
- [x] Run `go test ./risk ./cache ./testsuite -run 'TestEngine(RejectsMarketOrdersExceedingNotionalUsingQuoteTick|UsesQuoteTickMarksForProjectedAccountExposure)|TestCacheStoresLatestMarketEventsByInstrument|TestRiskTesterReportsOrderSafetyCases' -count=1`.

### Task 29: Portfolio Market Data Mark Closure

**Nautilus reference:**
- Portfolio unrealized PnL and exposure should update from normalized market data, not only from explicit manual mark setting.
- Quote ticks should mark long positions from bid and short positions from ask; order books use the same side-aware liquidation convention, while ticker/trade/bar data provide scalar fallback marks.
- Shared cache indexes must let portfolio update all account positions for the instrument carried by a market event.

**Files:**
- Modify: `cache/cache.go`
- Modify: `portfolio/portfolio.go`
- Modify: `testsuite/portfolio_tester.go`
- Test: `cache/cache_test.go`
- Test: `portfolio/portfolio_test.go`
- Test: `testsuite/portfolio_tester_test.go`

- [x] Add cache position enumeration by instrument.
- [x] Write failing coverage for portfolio unrealized PnL and exposure updates from quote ticks.
- [x] Implement `Portfolio.ApplyMarketEvent` with side-aware quote/order-book marks and scalar ticker/trade/bar fallbacks.
- [x] Add `TC-P04 Market data mark update` to the shared portfolio contract suite.
- [x] Run `go test ./portfolio ./cache ./testsuite -run 'TestPortfolioAppliesQuoteTickMarksForUnrealizedPnL|TestCacheStoresFillsPositionsAndDeduplicatesTrades|TestPortfolioTesterReportsPnLAndCommissionCases' -count=1`.

### Task 30: Platform Portfolio Market Data Wiring

**Nautilus reference:**
- Live/runtime nodes should route normalized market data into shared cache and portfolio state so strategy-facing portfolio PnL/exposure reflects current data without manual mark updates.
- Portfolio market data handling belongs in the platform runtime boundary, while the portfolio package owns mark selection and PnL math.

**Files:**
- Modify: `platform/node.go`
- Test: `platform/node_test.go`

- [x] Write failing coverage proving `platform.Node` market-data forwarding does not update portfolio marks.
- [x] Route market events through `Portfolio.ApplyMarketEvent` when a portfolio is configured, preserving cache-only behavior when it is not.
- [x] Run `go test ./platform ./portfolio -run 'TestNodeUpdatesPortfolioMarksFromMarketData|TestPortfolioAppliesQuoteTickMarksForUnrealizedPnL' -count=1`.

### Task 31: Backtest Portfolio Result Closure

**Nautilus reference:**
- Backtest results should expose portfolio state derived from simulated fills and replayed market data, not only raw cache state.
- Fill-derived execution events must still preserve position lifecycle ordering; portfolio accounting must not pre-mutate the runtime cache before position lifecycle derivation.
- Backtest portfolio accounting can use an isolated portfolio cache while the runtime cache remains the lifecycle source of truth.

**Files:**
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Add failing coverage for `backtest.Result.Portfolio` unrealized PnL and exposure after fills plus quote tick market data.
- [x] Add `Config.Cache`, `Config.Portfolio`, and `Result.Portfolio` to the backtest runner.
- [x] Create a default isolated portfolio cache for backtests and copy configured instruments for exposure grouping.
- [x] Route backtest fills and market events into portfolio accounting without breaking runtime position lifecycle events.
- [x] Run `go test ./backtest -run 'TestBacktest(ResultPortfolioUpdatesFromMarketData|MatchesMarketOrderAgainstTicker|DispatchesPositionLifecycleCallbacks|BracketReleasesChildrenAndCancelsOcoSibling)' -count=1`.

### Task 32: Backtest Shared Portfolio Cache Boundary

**Nautilus reference:**
- Execution/cache lifecycle state and portfolio accounting must remain consistent even when users wire the same cache instance through runtime and portfolio surfaces.
- A fill should be applied exactly once to portfolio accounting; runtime position lifecycle events should not be polluted by portfolio's own position projection.
- Backtest runtimes which already compute previous/next position snapshots should pass those snapshots into portfolio accounting instead of asking portfolio to infer state from a cache that may already contain the new position.

**Files:**
- Modify: `portfolio/portfolio.go`
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Write failing coverage for explicit shared `Config.Cache` + `Config.Portfolio` backtests where fills were double-applied.
- [x] Add `Portfolio.ApplyFillWithPosition` to account a fill from explicit previous/next position snapshots.
- [x] Use runtime previous/next position snapshots when backtest routes fills into portfolio accounting.
- [x] Run `go test ./backtest ./portfolio -run 'TestBacktestSharedPortfolioCacheDoesNotDoubleApplyFills|TestBacktestResultPortfolioUpdatesFromMarketData|TestPortfolio(TracksRealizedUnrealizedPnLAndCommissions|RealizedPnLWhenFillFlipsPosition)' -count=1`.

### Task 33: Strategy Runtime Portfolio Access

**Nautilus reference:**
- Strategies should be able to access portfolio state from their runtime context, not only through external node/result objects.
- Portfolio remains owned by platform/backtest runtime, but strategy code gets a direct read-oriented handle for exposure and PnL decisions.

**Files:**
- Modify: `strategy/engine.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`
- Test maintenance: `strategy/strategy_test.go`, `strategy/typed_test.go`

- [x] Write failing coverage for a backtest strategy reading `rt.Portfolio().Exposure(...)` after a simulated fill.
- [x] Add `Portfolio() *portfolio.Portfolio` to the strategy runtime contract.
- [x] Implement portfolio access on backtest runtime and platform node.
- [x] Update strategy fake runtimes for the expanded contract.
- [x] Run `go test ./backtest ./strategy ./platform -run 'TestBacktestRuntimeExposesPortfolioToStrategies|TestEngineDeliversSubscribedEvents|TestTypedStrategyProvidesNautilusStyleCallbacks|TestNodeUpdatesPortfolioMarksFromMarketData' -count=1`.

### Task 34: Data Contract Transient Stream Retry

**Nautilus reference:**
- Real market-data stream tests should distinguish unsupported/protocol failures from transient network disconnects.
- Public stream subscription validation should recover from one-off EOF/timeout conditions by reconnecting and retrying before declaring the venue contract failed.
- `ErrNotSupported` remains a capability skip, not a retryable failure.

**Files:**
- Modify: `testsuite/data_tester.go`
- Test: `testsuite/contracts_test.go`

- [x] Write failing coverage for a streaming data client whose first order-book subscribe returns `io.EOF` and whose retry succeeds after disconnect/connect.
- [x] Add transient stream retry around subscribe/unsubscribe using the repository transient live-network classifier.
- [x] Preserve unsupported market-data semantics as skipped cases.
- [x] Run `go test ./testsuite -run 'TestDataTester(RetriesTransientSubscribeEOF|ReportsNautilusStyleCaseResults)' -count=1`.

### Task 35: Strategy Runtime Clock And Timer Closure

**Nautilus reference:**
- `examples/backtest/example_02_use_clock_timer/strategy.py` uses `self.clock.set_timer(...)` in `on_start` and receives `TimeEvent` callbacks for periodic strategy work.
- `nautilus_trader/execution/engine.pyx` uses `self._clock.set_timer_ns(...)` for lifecycle maintenance timers and cancels those timers on stop.
- Go strategy runtime should expose the same clock/timer shape in both live and backtest paths: strategies set timers from `OnStart`, receive typed timer callbacks, and backtests advance timers on simulated event time rather than wall-clock sleeps.

**Files:**
- Add: `strategy/timer.go`
- Modify: `strategy/engine.go`
- Modify: `strategy/typed.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `strategy/typed_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`
- Test maintenance: `strategy/strategy_test.go`

- [x] Write failing coverage for typed strategy `OnTimer(context.Context, strategy.TimerEvent)` callbacks.
- [x] Write failing coverage for backtest timers firing on simulated time before the next market event.
- [x] Add `strategy.Clock`, `strategy.TimerEvent`, `Runtime.Clock`, `Runtime.SetTimer`, and `Runtime.CancelTimer`.
- [x] Subscribe the async strategy engine to `strategy.TopicTimer`.
- [x] Dispatch typed timer events to `OnTimer`.
- [x] Implement deterministic backtest timer scheduling with `(timestamp, name)` due-ordering and simulated `Clock().Now()`.
- [x] Publish backtest timer events to both the synchronous strategy engine and the bus fan-out.
- [x] Implement live/platform wall-clock timers on `platform.Node` and cancel all timers during `Node.Stop`.
- [x] Add platform coverage proving timers publish events and can be canceled from the node registry.
- [x] Run `go test ./strategy -run TestTypedStrategyDispatchesTimerCallbacks -count=1`.
- [x] Run `go test ./backtest -run TestRunnerDispatchesTimersOnSimulatedClockBeforeNextEvent -count=1`.
- [x] Run `go test ./platform -run TestNodeSetTimerPublishesTimerEventsAndCanCancel -count=1`.
- [x] Run `go test -count=1 ./strategy ./backtest ./platform ./live`.
- [x] Run `go test -run '^$' ./...`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./live ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./strategy ./live ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 36: Risk Missing Price Notional Closure

**Nautilus reference:**
- Nautilus instruments carry `max_notional` / `min_notional` constraints and expose `notional_value(...)` instead of silently treating missing prices as zero-valued orders.
- Nautilus portfolio exposure code logs and returns no exposure when no price exists for a position; risk gates should fail closed when a configured order-notional limit cannot be evaluated.
- Go risk checks should therefore reject market orders when `MaxOrderNotional` is configured but no order price or cached market price can estimate notional.

**Files:**
- Modify: `risk/risk.go`
- Test: `risk/risk_test.go`
- Modify: `testsuite/risk_tester.go`
- Test: `testsuite/risk_tester_test.go`

- [x] Write failing coverage for a market order with `MaxOrderNotional` configured and no price source.
- [x] Add shared RiskTester case `TC-R09 Missing market price notional rejection` using an order-notional-only engine.
- [x] Reject order-notional checks when no positive price can be estimated.
- [x] Run `go test ./risk -run TestEngineRejectsOrderNotionalWhenPriceUnavailable -count=1`.
- [x] Run `go test ./testsuite -run TestRiskTesterReportsOrderSafetyCases -count=1`.

### Task 37: Portfolio Signed Mark Values Closure

**Nautilus reference:**
- `Portfolio.mark_values(...)` returns per-currency mark-to-market values where longs contribute positive notional and shorts contribute negative notional.
- Mark values are distinct from gross exposure: gross exposure is absolute risk footprint, while signed mark values preserve long/short offset information for portfolio decisions.

**Files:**
- Modify: `portfolio/portfolio.go`
- Test: `portfolio/portfolio_test.go`
- Modify: `testsuite/portfolio_tester.go`
- Test: `testsuite/portfolio_tester_test.go`

- [x] Write failing coverage for long and short positions contributing signed mark values.
- [x] Add shared PortfolioTester case `TC-P05 Signed mark values`.
- [x] Add `Portfolio.MarkValue(accountID, instrumentID)` for signed instrument mark value.
- [x] Add `Portfolio.MarkValues(accountID)` for per-currency signed aggregation using settle currency first and quote currency as fallback.
- [x] Preserve existing `Portfolio.Exposure` as gross absolute exposure.
- [x] Run `go test ./portfolio -run TestPortfolioMarkValuesUseSignedLongShortContributions -count=1`.
- [x] Run `go test ./testsuite -run TestPortfolioTesterReportsPnLAndCommissionCases -count=1`.

### Task 38: GTD Expiry Semantics Closure

**Nautilus reference:**
- Nautilus order constructors require `expire_time_ns > 0` for `TimeInForce.GTD` and require no expire time for non-GTD orders.
- Nautilus market orders reject GTD, while limit-style and trigger/limit order types can carry GTD expiry.
- Expired orders emit `OrderExpired` lifecycle events and transition to terminal `EXPIRED` before later market data can fill them.

**Files:**
- Modify: `model/order.go`
- Test: `model/model_test.go`
- Test: `model/order_factory_test.go`
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`
- Modify: `platform/node.go`

- [x] Write failing model coverage for GTD expiry validation and market+GTD rejection.
- [x] Update order factory tests to create GTD limit orders with explicit expire time instead of allowing non-GTD expire times.
- [x] Require `SubmitOrder` GTD orders to carry an expire time after the UNIX epoch and reject expire times on non-GTD orders.
- [x] Add `ExpireTime` to normalized `OrderStatusReport` so accepted live/backtest reports retain order expiry semantics.
- [x] Preserve `ExpireTime` in platform report normalization and submit-from-report conversion.
- [x] Write failing backtest coverage proving a GTD order expires on simulated time before the next market event can fill it.
- [x] Expire open GTD orders in the backtest event loop before recording/matching the next market event, publishing order and lifecycle events.
- [x] Run `go test ./model -run 'TestSubmitOrderRequiresExpireTimeOnlyForGTD|TestSubmitOrderRejectsGTDMarketOrders|TestOrderFactoryCreatesGTDLimitOrderWithExpireTime|TestSubmitOrderSupportsProductionOrderSemantics' -count=1`.
- [x] Run `go test ./backtest -run TestBacktestExpiresGTDOrdersOnSimulatedClock -count=1`.

### Task 39: IOC/FOK Backtest Time-In-Force Closure

**Nautilus reference:**
- Nautilus order bookability excludes IOC/FOK orders from resting on the book.
- IOC limit orders should use immediately available liquidity and cancel any unfilled remainder before later market data can fill them.
- FOK limit orders should fill the full quantity immediately or cancel without partial fills.

**Files:**
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Write failing coverage proving an unfilled IOC limit order cannot remain open and fill on later market data.
- [x] Write failing coverage proving a FOK limit order with insufficient immediate liquidity cancels without partial fills.
- [x] Add immediate fillability precheck for FOK limit orders before applying fills.
- [x] Cancel remaining open quantity for IOC/FOK limit orders after the current immediate matching attempt.
- [x] Preserve existing GTC partial-fill and GTD expiry behavior.
- [x] Run `go test ./backtest -run 'TestBacktestCancels(UnfilledIOCBeforeLaterMarketData|FOKWhenFullQuantityUnavailable)' -count=1`.
- [x] Run `go test ./backtest -run 'TestBacktest(ExpiresGTDOrdersOnSimulatedClock|MatchesLimitOrderAgainstBook|PartiallyFillsOrderBookLiquidity)' -count=1`.

### Task 40: Backtest Auto-Cancel Lifecycle Closure

**Nautilus reference:**
- Simulator-generated cancellations are still order lifecycle events, not just silent state snapshots.
- IOC/FOK and market-order residual cancellations should emit terminal `OrderCanceled` lifecycle events with the previous open state retained for audit/replay.
- Strategies must be able to observe cancellation through the same execution event bus path used by live adapters and manual cancel commands.

**Files:**
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Write failing coverage proving IOC/FOK auto-cancel paths publish `OrderEventCanceled` lifecycle events.
- [x] Publish `OrderEventCanceled` lifecycle events after applying auto-canceled order reports.
- [x] Stamp auto-canceled reports with the simulated clock time for deterministic replay.
- [x] Preserve existing manual cancel, GTD expiry, and partial-fill behavior.
- [x] Run `go test ./backtest -run 'TestBacktestCancels(UnfilledIOCBeforeLaterMarketData|FOKWhenFullQuantityUnavailable)' -count=1`.
- [x] Run `go test ./backtest -run 'TestBacktest(CancelOrderDispatchesPendingCancelAndCanceled|CancelAllOrdersCancelsOpenOrdersForInstrument|ExpiresGTDOrdersOnSimulatedClock|MatchesLimitOrderAgainstBook|PartiallyFillsOrderBookLiquidity)' -count=1`.
- [x] Run `go test ./strategy -run TestTypedStrategyDispatchesNautilusLifecycleEvents -count=1`.
- [x] Run `go test ./testsuite -run TestLifecycleTesterReportsNautilusOrderLifecycleCases -count=1`.

### Task 41: Hyperliquid Spot Streaming And Private-Stream Closure

**Nautilus reference:**
- Product surfaces should be separate but experience-consistent: spot and perp clients expose the same platform-level streaming and private execution lifecycle when the venue protocol supports it.
- Public BBO/L2 streams belong to the data client and emit normalized market events.
- Private order and fill streams belong to the execution client, remain connected across submissions, and expose explicit resubscribe behavior after reconnect.

**Files:**
- Modify: `adapter/hyperliquid/adapter.go`
- Modify: `adapter/hyperliquid/spot.go`
- Test: `adapter/hyperliquid/hyperliquid_test.go`

- [x] Write failing coverage requiring Hyperliquid Spot data clients to implement `venue.StreamingDataClient` for BBO ticker and L2 order book events.
- [x] Write failing coverage requiring Hyperliquid Spot execution clients to implement private order/fill stream mapping and `venue.ExecutionResubscriber`.
- [x] Add spot raw-symbol indexing for WS coin-to-instrument mapping.
- [x] Wire Hyperliquid Spot public BBO/L2 subscriptions into normalized `MarketEvent` output.
- [x] Wire Hyperliquid Spot private `orderUpdates` and `userFills` into normalized `ExecutionEvent` order and fill reports.
- [x] Upgrade Hyperliquid Spot capability claims to stream/private-stream after tests prove the object-level clients implement the contracts.
- [x] Run `go test ./adapter/hyperliquid -run 'TestSpot(DataClientStreamsBboAndOrderBook|ExecutionClientPrivateStreamMapsOrdersAndFills)' -count=1`.
- [x] Run `go test ./adapter/hyperliquid -count=1`.
- [x] Run `go test ./venue ./testsuite -count=1`.
- [x] Run `go test ./platform ./live -count=1`.

### Task 42: Hyperliquid Spot Order-Report Reconciliation Closure

**Nautilus reference:**
- Startup reconciliation depends on real open-order status reports, not empty success placeholders.
- If an adapter declares `Execution.OrderReports=true`, `GenerateOrderStatusReports` must query the venue or explicitly fail; returning `nil, nil` hides unreconciled live orders.
- SDK parity should expose the native venue read method first, then adapter code should normalize it.

**Files:**
- Modify: `sdk/hyperliquid/spot/types.go`
- Modify: `sdk/hyperliquid/spot/order.go`
- Test: `sdk/hyperliquid/spot/order_test.go`
- Modify: `adapter/hyperliquid/spot.go`
- Test: `adapter/hyperliquid/hyperliquid_test.go`

- [x] Write failing SDK coverage for Hyperliquid Spot `UserOpenOrders`.
- [x] Write failing adapter coverage proving Spot `GenerateOrderStatusReports` returns normalized open-order reports instead of an empty placeholder.
- [x] Add Spot SDK `Order` DTO and `UserOpenOrders(ctx, user)` wrapper over Hyperliquid `openOrders`.
- [x] Normalize Spot open orders into `OrderStatusReport` with side, quantity, filled/leaves, price, status, and update time.
- [x] Run `go test ./sdk/hyperliquid/spot -run TestClient_UserOpenOrders -count=1`.
- [x] Run `go test ./adapter/hyperliquid -run TestSpotGenerateOrderStatusReportsUsesOpenOrders -count=1`.
- [x] Run `go test ./sdk/hyperliquid ./sdk/hyperliquid/spot ./sdk/hyperliquid/perp -count=1`.
- [x] Run `go test ./adapter/hyperliquid -count=1`.
- [x] Run `go test ./adapter/hyperliquid -run 'TestSpot(ClientsPassVenueContractSuite|DataClientStreamsBboAndOrderBook|ExecutionClientPrivateStreamMapsOrdersAndFills|GenerateOrderStatusReportsUsesOpenOrders)' -count=1`.

### Task 43: Adapter Contract Test Isolation Closure

**Nautilus reference:**
- Nautilus-style adapter contract tests must be deterministic and should not accidentally connect to real venue infrastructure unless explicitly marked as live tests.
- Shared contract suites should still exercise streaming interfaces, but fake SDK adapter tests must inject fake stream transports.
- Live networking belongs in separately gated SDK/live tests, not default adapter unit tests.

**Files:**
- Modify: `adapter/hyperliquid/hyperliquid_test.go`
- Modify: `adapter/lighter/lighter_test.go`
- Modify: `adapter/okx/okx_test.go`

- [x] Use `go test -count=1 ./adapter/...` failure as red coverage for contract suites leaking to real WS endpoints.
- [x] Inject fake Hyperliquid Spot/Perp market WS clients into venue contract tests.
- [x] Inject fake Lighter market WS client into venue contract tests.
- [x] Inject fake OKX Spot/Swap public WS clients into venue contract tests.
- [x] Preserve contract coverage of subscribe/unsubscribe behavior while removing accidental external networking from default adapter tests.
- [x] Run `go test ./adapter/hyperliquid ./adapter/lighter ./adapter/okx -count=1`.

### Task 44: Portfolio Account Equity And Margin Closure

**Nautilus reference:**
- `PortfolioFacade` exposes account-facing balance, margin, PnL, mark-value, and equity queries.
- `AccountBalance` requires `total - locked == free`.
- `MarginBalance` carries non-negative initial and maintenance margin, optionally scoped to an instrument.
- Portfolio equity uses account balances plus open-position value/PnL, and margin availability must account for locked balances and initial margin.

**Files:**
- Modify: `model/account.go`
- Test: `model/model_test.go`
- Modify: `portfolio/portfolio.go`
- Test: `portfolio/portfolio_test.go`
- Modify: `testsuite/portfolio_tester.go`
- Test: `testsuite/portfolio_tester_test.go`

- [x] Write failing model coverage for balance accounting invariants and margin validation.
- [x] Write failing portfolio coverage for account locked balances, initial/maintenance margin, equity, and available equity.
- [x] Add shared `TC-P06 Account equity and margins` to the portfolio contract suite.
- [x] Add account type, margin balance, balance amount parsing, and account snapshot validation.
- [x] Add Portfolio account update, locked balance, margin, PnL aggregate, equity, and available-equity query surfaces.
- [x] Run `go test ./model -run TestAccountSnapshotValidatesBalancesAndMargins -count=1`.
- [x] Run `go test ./portfolio -run TestPortfolioTracksAccountBalancesMarginsAndEquity -count=1`.
- [x] Run `go test ./testsuite -run TestPortfolioTesterReportsPnLAndCommissionCases -count=1`.
- [x] Run `go test ./platform -run 'TestNode(QueryAccount|UpdatesPortfolio|RecoversPrivateStream)' -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 ./adapter/...`.

### Task 45: Risk Initial-Margin Headroom Closure

**Nautilus reference:**
- Futures, perps, and options carry initial and maintenance margin requirements at the instrument/accounting boundary.
- Risk and account handling must reject orders when account margin headroom is insufficient, not only when a user-configured notional cap is exceeded.
- Margin account availability is driven by account balances, locked funds, existing margin requirements, and mark-to-market unrealized PnL.

**Files:**
- Modify: `model/instrument.go`
- Test: `model/model_test.go`
- Modify: `risk/risk.go`
- Test: `risk/risk_test.go`
- Modify: `testsuite/risk_tester.go`
- Test: `testsuite/risk_tester_test.go`

- [x] Write failing model coverage for non-negative optional instrument margin rates.
- [x] Write failing risk coverage for available-initial-margin rejection.
- [x] Add shared `TC-R10 Available initial margin rejection` to the risk contract suite.
- [x] Add optional `Instrument.MarginInit` and `Instrument.MarginMaint`.
- [x] Make risk checks compute margin headroom from account balances, locked balances, existing initial margins, and unrealized PnL.
- [x] Reject exposure-increasing orders whose incremental initial margin exceeds available margin.
- [x] Run `go test ./model -run TestInstrumentValidateAllowsNonNegativeMarginRates -count=1`.
- [x] Run `go test ./risk -run TestEngineRejectsOrdersExceedingAvailableInitialMargin -count=1`.
- [x] Run `go test ./testsuite -run TestRiskTesterReportsOrderSafetyCases -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 ./adapter/...`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./model ./risk ./portfolio ./platform ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 46: SDK-Backed Instrument Fee And Margin Metadata Closure

**Nautilus reference:**
- Instrument providers should expose venue trading metadata, not only symbol precision.
- Futures/perp instruments carry maker/taker fee rates and initial/maintenance margin requirements where the venue SDK supplies them.
- Shared venue contract tests should be able to assert normalized metadata so this does not regress per adapter.

**Files:**
- Modify: `model/instrument.go`
- Test: `model/model_test.go`
- Modify: `testsuite/contracts.go`
- Test: `testsuite/contracts_test.go`
- Modify: `adapter/edgex/adapter.go`
- Modify: `adapter/edgex/helpers.go`
- Test: `adapter/edgex/edgex_test.go`
- Modify: `adapter/hyperliquid/perp.go`
- Test: `adapter/hyperliquid/hyperliquid_test.go`

- [x] Write failing model coverage for non-negative maker/taker fee rates.
- [x] Write failing EdgeX provider coverage requiring fee and margin metadata from SDK exchange info.
- [x] Add optional metadata expectations to `VenueContractConfig`.
- [x] Add testsuite coverage proving venue contract suites can assert expected fee/margin metadata.
- [x] Map EdgeX SDK `DefaultMakerFeeRate`, `DefaultTakerFeeRate`, risk-tier `MaxLeverage`, and `MaintenanceMarginRate` into normalized instruments.
- [x] Map Hyperliquid perp `maxLeverage` into normalized initial margin.
- [x] Keep EdgeX venue contract tests on fake market WS transport so default/race tests do not start SDK websocket goroutines.
- [x] Run `go test ./model -run TestInstrumentValidateAllowsNonNegativeFeesAndMarginRates -count=1`.
- [x] Run `go test ./adapter/edgex -run TestInstrumentProviderNormalizesFeeAndMarginMetadata -count=1`.
- [x] Run `go test ./testsuite -run TestVenueContractSuiteChecksExpectedInstrumentMetadata -count=1`.
- [x] Run `go test ./adapter/edgex -count=1`.
- [x] Run `go test ./adapter/hyperliquid -run TestPerpClientsPassVenueContractSuite -count=1`.
- [x] Run `go test -race -count=1 ./adapter/edgex ./adapter/hyperliquid`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 ./adapter/...`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./model ./risk ./portfolio ./platform ./testsuite ./adapter/edgex ./adapter/hyperliquid`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 47: Multi-Exchange SDK Instrument Metadata Closure

**Nautilus reference:**
- Instrument providers are authoritative normalized metadata sources for risk, portfolio, and matching engines.
- Venue SDK fee, leverage, and margin-weight fields must be normalized before runtime logic sees them.
- Shared venue contract suites should assert adapter-specific metadata whenever the SDK provides it.

**Files:**
- Modify: `adapter/lighter/adapter.go`
- Modify: `adapter/lighter/helpers.go`
- Test: `adapter/lighter/lighter_test.go`
- Modify: `adapter/standx/adapter.go`
- Modify: `adapter/standx/helpers.go`
- Test: `adapter/standx/standx_test.go`
- Modify: `adapter/bitget/provider.go`
- Test: `adapter/bitget/bitget_test.go`
- Modify: `adapter/nado/adapter.go`
- Test: `adapter/nado/nado_test.go`

- [x] Write failing Lighter provider coverage for SDK maker/taker fee and margin fractions.
- [x] Write failing StandX provider coverage for SDK maker/taker fee and max-leverage initial margin.
- [x] Write failing Bitget provider coverage for SDK spot/perp maker/taker fees.
- [x] Write failing Nado provider coverage for SDK X18 maker/taker fees and margin weights.
- [x] Add venue contract metadata expectations for Lighter, StandX, Bitget, and Nado.
- [x] Map Lighter `maker_fee`, `taker_fee`, `default_initial_margin_fraction`, and `maintenance_margin_fraction`.
- [x] Map StandX `maker_fee`, `taker_fee`, and `max_leverage`.
- [x] Map Bitget `makerFeeRate` and `takerFeeRate` for spot and USDT futures instruments.
- [x] Load Nado `GetSymbols` metadata and map X18 fee rates plus long initial/maintenance weights.
- [x] Run `go test ./adapter/lighter -run TestInstrumentProviderNormalizesFeeAndMarginMetadata -count=1`.
- [x] Run `go test ./adapter/standx -run TestInstrumentProviderNormalizesFeeAndMarginMetadata -count=1`.
- [x] Run `go test ./adapter/bitget -run TestInstrumentProviderNormalizesFeeMetadata -count=1`.
- [x] Run `go test ./adapter/nado -run TestInstrumentProviderNormalizesFeeAndMarginMetadata -count=1`.
- [x] Run `go test ./adapter/lighter -count=1`.
- [x] Run `go test ./adapter/standx -count=1`.
- [x] Run `go test ./adapter/bitget -count=1`.
- [x] Run `go test ./adapter/nado -count=1`.
- [x] Run `go test -count=1 ./adapter/...`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./model ./risk ./portfolio ./testsuite ./adapter/lighter ./adapter/standx ./adapter/bitget ./adapter/nado ./adapter/edgex ./adapter/hyperliquid`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 48: Risk Open-Order Exposure And Margin Closure

**Nautilus reference:**
- Risk checks account for active orders, not only settled positions.
- Open order leaves are potential future exposure and should count toward projected position/account limits.
- Open order leaves reserve initial margin headroom until canceled, filled, expired, or otherwise terminal.
- Reduce-only open orders should not be used to release risk capacity before execution.

**Files:**
- Modify: `risk/risk.go`
- Test: `risk/risk_test.go`
- Modify: `testsuite/risk_tester.go`
- Test: `testsuite/risk_tester_test.go`

- [x] Write failing risk coverage proving open orders are missing from projected position notional.
- [x] Write failing risk coverage proving open orders are missing from available initial-margin headroom.
- [x] Add shared `TC-R11 Open order projected position rejection`.
- [x] Add shared `TC-R12 Open order initial margin rejection`.
- [x] Include non-reduce-only open order leaves in projected signed positions.
- [x] Include open order leaves in account exposure projections with mark/entry/order-price fallback.
- [x] Reserve initial margin for non-reduce-only open order leaves before admitting new exposure.
- [x] Keep reduce-only checks tied to current position so open reduce-only orders do not release risk capacity.
- [x] Run `go test ./risk -run 'TestEngineRejectsOrdersExceedingProjectedPositionNotionalIncludingOpenOrders|TestEngineRejectsOrdersExceedingAvailableInitialMarginIncludingOpenOrders' -count=1`.
- [x] Run `go test ./testsuite -run TestRiskTesterReportsOrderSafetyCases -count=1`.
- [x] Run `go test ./risk -count=1`.
- [x] Run `go test ./testsuite -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./risk ./testsuite ./cache ./platform ./backtest ./portfolio`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 49: Reconnect Missing-Open-Order Reconciliation Closure

**Nautilus reference:**
- Reconciliation after a private-stream reconnect must repair local order state, not only append venue reports.
- Local open orders missing from the venue snapshot must not remain open forever because they continue to affect cache, risk, and user-visible strategy state.
- Generated terminal reports should pass through the same account cache and platform event surfaces as venue reports.

**Files:**
- Modify: `account/reconciler.go`
- Test: `account/reconciler_test.go`
- Modify: `platform/node.go`
- Test: `platform/node_test.go`

- [x] Write failing reconciler coverage for a local open order missing from an order snapshot.
- [x] Write failing platform recovery coverage proving a missing local open order is canceled after reconnect reconciliation.
- [x] Add `ReconcileMissingOpenOrders` to diff local open orders against venue order/client/venue ids.
- [x] Generate terminal canceled reports for missing local open orders through the account reconciler.
- [x] Publish generated missing-order reports from `Node.reconcileInstrument`.
- [x] Run `go test ./account -run TestReconcilerMarksOpenOrdersMissingFromSnapshotCanceled -count=1`.
- [x] Run `go test ./platform -run TestNodeRecoveryCancelsLocalOpenOrdersMissingFromVenueSnapshot -count=1`.
- [x] Run `go test ./account -count=1`.
- [x] Run `go test ./platform -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./account ./platform ./cache ./risk ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 50: Reconnect Missing-Position Reconciliation Closure

**Nautilus reference:**
- Position reconciliation after reconnect must repair stale local positions, not only apply returned venue positions.
- A local non-flat position missing from a supported venue position snapshot should become a flat position report.
- Generated flat reports must flow through the normal platform path so position lifecycle events and downstream portfolio/risk state remain consistent.

**Files:**
- Modify: `account/reconciler.go`
- Test: `account/reconciler_test.go`
- Modify: `platform/node.go`
- Test: `platform/node_test.go`

- [x] Write failing reconciler coverage for a local non-flat position missing from a position snapshot.
- [x] Write failing platform recovery coverage proving a missing local position is flattened after reconnect reconciliation.
- [x] Add `MissingPositionReports` to generate flat reports for stale local positions.
- [x] Only flatten missing positions when `GeneratePositionStatusReports` is supported and succeeds.
- [x] Route generated flat reports through `Node.applyAndPublish` to preserve position lifecycle derivation.
- [x] Run `go test ./account -run TestReconcilerGeneratesFlatPositionReportsMissingFromSnapshot -count=1`.
- [x] Run `go test ./platform -run TestNodeRecoveryFlattensLocalPositionsMissingFromVenueSnapshot -count=1`.
- [x] Run `go test ./account -count=1`.
- [x] Run `go test ./platform -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./account ./platform ./cache ./portfolio ./risk ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 51: Market-Data Stream Reconnect And Resubscribe Closure

**Nautilus reference:**
- Data subscriptions are runtime state; reconnecting a streaming data client must restore active subscriptions.
- Market-data event forwarding should recover from stream-channel closure in the same lifecycle spirit as private execution streams.
- Subscription replay must preserve full subscription objects, including bar type and order-book depth, not only venue/client identity.

**Files:**
- Modify: `platform/node.go`
- Test: `platform/node_test.go`

- [x] Write failing platform coverage proving market-data stream closure does not reconnect/resubscribe.
- [x] Store active market-data subscription snapshots separately from key-to-client routing.
- [x] Reconnect data clients whose event channel closes while the node is still running.
- [x] Replay active subscriptions for the recovered streaming data client.
- [x] Add fake data-client channel replacement with race-safe call capture.
- [x] Run `go test ./platform -run TestNodeRecoversMarketDataStreamAndResubscribesActiveSubscriptions -count=1`.
- [x] Run `go test ./platform -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./platform ./cache ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 52: Backtest Fill Fee And Portfolio Commission Closure

**Nautilus reference:**
- Backtest execution reports should carry realistic fill economics, not only price and quantity.
- Simulated fills should derive maker/taker fees from instrument metadata and emit fee currency so account and portfolio accounting can reconcile commissions deterministically.
- Market-style/immediate fills are taker; post-only or resting limit-style fills use maker economics.

**Files:**
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Write failing backtest coverage proving market fills ignored instrument taker fee and portfolio commission stayed zero.
- [x] Write failing backtest coverage proving resting post-only limit fills ignored instrument maker fee.
- [x] Add accepted-order timestamps so resting-vs-immediate simulated fills can be classified.
- [x] Route all backtest fill construction through one helper that assigns trade IDs and applies instrument maker/taker fees.
- [x] Use settle currency for derivatives and quote currency for spot fill fees.
- [x] Verify fill fees propagate through `Portfolio.ApplyFillWithPosition` into commission totals.
- [x] Run `go test ./backtest -run 'TestBacktestAppliesInstrument(Taker|Maker)Fee' -count=1`.
- [x] Run `go test ./backtest -count=1`.
- [x] Run `go test ./portfolio -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./backtest ./portfolio ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.

### Task 53: Capability-Aware Venue Contract Strictness

**Nautilus reference:**
- Test suites must fail when a venue claims a capability but only returns skipped/unsupported behavior.
- Capability declarations must distinguish snapshot/read support from streaming support, and fixture-gated private streams must be explicitly required rather than silently inferred from no-credential contract tests.
- Ticker and order-book subscriptions are first-class data contract cases; trade, quote, and bar streams remain capability-specific claims until each venue exposes real SDK-backed stream support.

**Files:**
- Modify: `venue/capabilities.go`
- Modify: `testsuite/report.go`
- Modify: `testsuite/contracts.go`
- Modify: `testsuite/data_tester.go`
- Test: `testsuite/contracts_test.go`
- Test: `adapter/*/*_test.go`

- [x] Write failing tests proving skipped data/execution cases should be rejected by strict `AllPassed` semantics.
- [x] Add `ContractReport.AllPassed` and `RequiredPassed` so optional skip reporting and strict declared-capability enforcement are separate.
- [x] Add `TC-D11` ticker subscription coverage to `DataTester`.
- [x] Split market-data capabilities into read and stream fields: `Ticker`, `OrderBook`, `TickerStream`, `OrderBookStream`, plus capability-specific trade/quote/bar stream flags.
- [x] Make `RunVenueContractSuite` require only cases declared by the supplied fixture capabilities.
- [x] Keep private-stream `TC-E84` as an explicit fixture requirement so no-credential contract tests do not hide missing fake private stream setup.
- [x] Update adapter capability declarations and contract tests to pass fixture capabilities.
- [x] Run `go test ./testsuite -count=1`.
- [x] Run `go test -count=1 ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -run '^$' ./sdk/...`.

### Task 54: Binance Full Public Data Stream Capability

**Nautilus reference:**
- Public market-data clients should expose normalized stream types, not only REST snapshots or order-book streams.
- Quote ticks, trade ticks, and bars must be SDK-backed venue subscriptions with normalized `MarketEvent` payloads.
- Shared venue streams such as Binance bookTicker should fan out to active ticker and quote subscriptions without overwriting one subscriber with another.

**Files:**
- Modify: `adapter/binance/adapter.go`
- Modify: `adapter/binance/spot.go`
- Modify: `adapter/binance/perp.go`
- Create: `adapter/binance/market_helpers.go`
- Test: `adapter/binance/spot_test.go`
- Test: `adapter/binance/perp_test.go`

- [x] Write failing Binance spot/perp contract coverage by declaring `TradeTicks`, `QuoteTicks`, and `Bars` stream capabilities.
- [x] Write failing spot/perp data-client tests for bookTicker quote fan-out, aggTrade trade ticks, and 1m kline bars.
- [x] Implement shared bookTicker subscription fan-out for ticker and quote ticks.
- [x] Map Binance aggTrade streams to normalized `model.TradeTick` with aggressor side and trade IDs.
- [x] Map Binance kline streams to normalized external time bars.
- [x] Add subscribe/unsubscribe paths and fake stream coverage for spot and perp.
- [x] Run `go test ./adapter/binance -run 'Test(Spot|Perp)(ClientsPassVenueContractSuite|DataClientStreamsTickerAndOrderBook)' -count=1`.
- [x] Run `go test ./adapter/binance -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./adapter/binance ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -run '^$' ./sdk/...`.

### Task 55: Aster Full Public Data Stream Capability

**Nautilus reference:**
- Each venue adapter should expose declared public stream capabilities through real SDK-backed subscriptions and normalized market events.
- Shared best bid/ask streams should fan out to ticker and quote subscriptions without replacing active subscribers.
- Trade ticks and bars should normalize venue agg-trade and kline payloads into stable `model.TradeTick` and `model.Bar` events.

**Files:**
- Modify: `adapter/aster/adapter.go`
- Modify: `adapter/aster/helpers.go`
- Modify: `adapter/aster/spot.go`
- Modify: `adapter/aster/perp.go`
- Test: `adapter/aster/aster_test.go`

- [x] Write failing Aster spot/perp contract coverage by declaring `TradeTicks`, `QuoteTicks`, and `Bars` stream capabilities.
- [x] Write failing spot/perp data-client tests for bookTicker quote fan-out, aggTrade trade ticks, and 1m kline bars.
- [x] Implement shared bookTicker subscription fan-out for ticker and quote ticks.
- [x] Map Aster aggTrade streams to normalized `model.TradeTick` with aggressor side and trade IDs.
- [x] Map Aster kline streams to normalized external time bars.
- [x] Add subscribe/unsubscribe paths and fake stream coverage for spot and perp.
- [x] Run `go test ./adapter/aster -run 'Test(Spot|Perp)(ClientsPassVenueContractSuite|DataClientStreamsTickerAndOrderBook)' -count=1`.
- [x] Run `go test ./adapter/aster -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./adapter/aster ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -run '^$' ./sdk/...`.

### Task 56: Bybit Full Public Data Stream Capability

**Nautilus reference:**
- Venue stream capabilities should be declared only after the adapter exposes real normalized stream payloads.
- Shared underlying topics must support multiple logical subscriptions without overwriting handlers or prematurely unsubscribing the other logical subscription.
- Product-specific protocol differences matter: Bybit linear can derive quote ticks from ticker payloads, while spot quote ticks need top-of-book data from `orderbook.1`.

**Files:**
- Modify: `adapter/bybit/adapter.go`
- Modify: `adapter/bybit/data.go`
- Modify: `adapter/bybit/helpers.go`
- Test: `adapter/bybit/bybit_test.go`

- [x] Write failing Bybit data-client tests for linear quote tick fan-out, `publicTrade` trade ticks, 1m kline bars, and spot top-of-book quote ticks.
- [x] Declare Bybit `TradeTicks`, `QuoteTicks`, and `Bars` only after the data client supports the corresponding subscriptions.
- [x] Add topic reference tracking so ticker/quote and orderbook/quote shared topics do not unsubscribe each other.
- [x] Map Bybit `publicTrade` payloads to normalized `model.TradeTick` with aggressor side and trade IDs.
- [x] Map Bybit kline payloads to normalized external time bars with revision flag semantics.
- [x] Map Bybit linear ticker best bid/ask sizes and spot `orderbook.1` top of book to normalized `model.QuoteTick`.
- [x] Run `go test ./adapter/bybit -run 'Test(Spot|Linear)ClientsPassVenueContractSuite|TestDataClientStreams(TickerAndOrderBook|NautilusMarketDataTypes)|TestSpotDataClientStreamsQuoteTickFromTopOfBook' -count=1`.
- [x] Run `go test ./adapter/bybit -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./adapter/bybit ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 57: Bitget Full Public Data Stream Capability And Nado Contract Isolation

**Nautilus reference:**
- Public data stream contracts must be deterministic local adapter tests unless a test is explicitly marked as live.
- Logical market-data subscriptions such as ticker and quote ticks may share a venue stream, but the adapter must preserve independent subscribe/unsubscribe semantics.
- Venue public fill and candle streams should be normalized to `model.TradeTick` and external time bars before entering strategy/runtime surfaces.

**Files:**
- Modify: `adapter/bitget/adapter.go`
- Modify: `adapter/bitget/data.go`
- Modify: `adapter/bitget/helpers.go`
- Test: `adapter/bitget/bitget_test.go`
- Test: `adapter/nado/nado_test.go`

- [x] Write failing Bitget tests for ticker/quote shared-stream fan-out, no premature unsubscribe, trade ticks, and 1m candle bars.
- [x] Declare Bitget `TradeTicks`, `QuoteTicks`, and `Bars` only after implementing corresponding subscriptions.
- [x] Add Bitget logical-subscription to `WSArg` tracking so ticker and quote share one ticker channel safely.
- [x] Map Bitget `trade` channel fills to normalized `model.TradeTick` with aggressor side and trade IDs.
- [x] Map Bitget `candle1m` payloads to normalized external time bars.
- [x] Fix Nado venue contract fixture to use the existing fake market WS instead of connecting to the live Nado public stream during default tests.
- [x] Run `go test ./adapter/bitget -run 'Test(Spot|Perp)ClientsPassVenueContractSuite|TestDataClientStreams(TickerAndOrderBook|NautilusMarketDataTypes)' -count=1`.
- [x] Run `go test ./adapter/bitget -count=1`.
- [x] Run `go test ./adapter/nado -run TestPerpClientsPassVenueContractSuite -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./adapter/bitget ./adapter/nado ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 58: OKX Full Public Data Stream Capability

**Nautilus reference:**
- SDK typed helpers should own venue WS payload decoding; adapters should consume typed SDK callbacks and normalize them into platform `MarketEvent` payloads.
- Ticker-derived quote ticks must share the underlying ticker stream while preserving independent logical subscription lifecycle.
- Candle streams should carry revision semantics so unfinished bars are distinguishable from final bars.

**Files:**
- Modify: `sdk/okx/ws_market.go`
- Test: `sdk/okx/ws_market_test.go`
- Modify: `adapter/okx/adapter.go`
- Modify: `adapter/okx/data.go`
- Modify: `adapter/okx/helpers.go`
- Test: `adapter/okx/okx_test.go`

- [x] Write failing OKX adapter tests for ticker/quote shared-stream fan-out, no premature unsubscribe, trades, and 1m candles.
- [x] Add SDK typed public WS helpers `SubscribeTrades` and `SubscribeCandles` over the existing raw `WSClient.Subscribe`.
- [x] Add OKX adapter logical-subscription to `WsSubscribeArgs` tracking for ticker/quote stream sharing.
- [x] Map OKX public trades to normalized `model.TradeTick` with aggressor side and trade IDs.
- [x] Map OKX candle payloads to normalized external time bars with unfinished-candle revision semantics.
- [x] Declare OKX `TradeTicks`, `QuoteTicks`, and `Bars` only after adapter contract coverage passes.
- [x] Run `go test ./adapter/okx -run 'Test(Spot|Swap)ClientsPassVenueContractSuite|TestDataClientStreams(TickerAndOrderBook|NautilusMarketDataTypes)' -count=1`.
- [x] Run `go test ./adapter/okx -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./adapter/okx ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `go test ./sdk/okx -run 'TestWSClientConstructorCompatibility' -count=1`.
- [x] Run `git diff --check`.

### Task 59: Nado Full Public Data Stream Capability

**Nautilus reference:**
- Public stream capability claims must be backed by deterministic adapter contract tests, not live-network behavior in default test runs.
- Quote ticks should use venue top-of-book fields with real sizes; when ticker streams provide bid/ask quantities, they can safely fan out to ticker and quote events.
- Venue-specific numeric encodings such as X18 prices must be normalized before reaching the platform model.

**Files:**
- Modify: `adapter/nado/adapter.go`
- Test: `adapter/nado/nado_test.go`

- [x] Write failing Nado tests for ticker/quote shared-stream fan-out, no premature unsubscribe, trade ticks, and latest candlestick bars.
- [x] Add Nado logical topic tracking so ticker and quote share the SDK best-bid-offer stream safely.
- [x] Map Nado trade stream payloads to normalized `model.TradeTick`, including fallback trade IDs for ID-less public trade payloads.
- [x] Map Nado latest candlestick stream payloads to normalized bars with X18 price conversion.
- [x] Declare Nado `TradeTicks`, `QuoteTicks`, and `Bars` only after contract coverage passes.
- [x] Run `go test ./adapter/nado -run 'TestPerpClientsPassVenueContractSuite|TestDataClientStreams(TickerAndOrderBook|NautilusMarketDataTypes)' -count=1`.
- [x] Run `go test ./adapter/nado -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./adapter/nado ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 60: Edgex Full Public Data Stream Capability

**Nautilus reference:**
- Public market-data capability claims should reflect real independently subscribable normalized event streams.
- Quote ticks can be derived from deterministic top-of-book depth updates when the venue does not expose a separate quote stream.
- Candle/K-line streams must preserve venue interval and price-type routing while presenting normalized bar timestamps and values to strategies.

**Files:**
- Modify: `adapter/edgex/adapter.go`
- Modify: `adapter/edgex/helpers.go`
- Test: `adapter/edgex/edgex_test.go`

- [x] Write failing Edgex tests for quote ticks from top-of-book, public trades, and 1m K-line bars.
- [x] Extend the Edgex market WS adapter seam with trade and K-line subscriptions while keeping tests on deterministic fakes.
- [x] Add Edgex logical topic tracking for ticker, book, quote, trade, and bar subscriptions.
- [x] Map depth-15 top-of-book updates to normalized `model.QuoteTick`.
- [x] Map public trade payloads to normalized `model.TradeTick` with venue trade IDs and aggressor side.
- [x] Map K-line stream payloads to normalized last-price time bars with interval conversion.
- [x] Declare Edgex `TradeTicks`, `QuoteTicks`, and `Bars` only after adapter contract coverage passes.
- [x] Run `go test ./adapter/edgex -run 'TestPerpClientsPassVenueContractSuite|TestDataClientStreams(TickerAndOrderBook|NautilusMarketDataTypes)' -count=1`.
- [x] Run `go test ./adapter/edgex -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./adapter/edgex ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 61: GRVT Full Public Data Stream Capability

**Nautilus reference:**
- Adapter capability declarations must be executable contracts: when `TradeTicks`, `QuoteTicks`, or `Bars` are true, the shared venue suite must require the corresponding subscription cases.
- Quote ticks can share a best-bid-offer ticker stream when the venue ticker payload includes bid/ask sizes.
- Public trade and candle streams should preserve venue trade IDs, aggressor direction, bar interval, and OHLCV data in normalized model events.

**Files:**
- Modify: `adapter/grvt/adapter.go`
- Modify: `adapter/grvt/helpers.go`
- Test: `adapter/grvt/grvt_test.go`

- [x] Write failing GRVT tests for ticker/quote shared-stream fan-out, no premature unsubscribe, trade ticks, and 1m candle bars.
- [x] Add GRVT logical topic tracking so ticker and quote share the SDK ticker snap stream safely.
- [x] Map GRVT public trade payloads to normalized `model.TradeTick` with venue trade IDs and taker-side aggressor mapping.
- [x] Map GRVT K-line stream payloads to normalized external last-price bars with interval conversion.
- [x] Declare GRVT `TradeTicks`, `QuoteTicks`, and `Bars` only after adapter contract coverage passes.
- [x] Run `go test ./adapter/grvt -run TestDataClientStreamsNautilusMarketDataTypes -count=1`.
- [x] Run `go test ./adapter/grvt -run 'TestPerpClientsPassVenueContractSuite|TestDataClientStreams(TickerAndOrderBook|NautilusMarketDataTypes)' -count=1`.
- [x] Run `go test ./adapter/grvt -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./adapter/grvt ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 62: Lighter Public Quote And Trade Stream Capability

**Nautilus reference:**
- Public market-data capability claims must stay honest: enable normalized streams that are backed by real venue WebSocket subscriptions, and keep unsupported bar streams unclaimed.
- Quote ticks can share a best-bid-offer ticker stream when the ticker payload carries bid/ask quantities.
- Public trade streams should map venue trade IDs, prices, sizes, timestamps, and aggressor direction into normalized `TradeTick` events.

**Files:**
- Modify: `adapter/lighter/adapter.go`
- Modify: `adapter/lighter/helpers.go`
- Test: `adapter/lighter/lighter_test.go`

- [x] Write failing Lighter tests for ticker/quote shared-stream fan-out, no premature unsubscribe, and public trade ticks.
- [x] Add Lighter logical topic tracking so ticker and quote share the SDK ticker stream safely.
- [x] Map Lighter public trade stream payloads to normalized `model.TradeTick`, including liquidation trade rows.
- [x] Declare Lighter `TradeTicks` and `QuoteTicks`; keep `Bars` unclaimed because the current SDK exposes candlesticks through REST, not a public candle WebSocket stream.
- [x] Run `go test ./adapter/lighter -run TestDataClientStreamsNautilusMarketDataTypes -count=1`.
- [x] Run `go test ./adapter/lighter -run 'TestPerpClientsPassVenueContractSuite|TestDataClientStreams(TickerAndOrderBook|NautilusMarketDataTypes)' -count=1`.
- [x] Run `go test ./adapter/lighter -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -race -count=1 ./adapter/lighter ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 63: Hyperliquid Full Public Data Stream Capability

**Nautilus reference:**
- Public stream capability claims must be backed by real venue WebSocket subscriptions and shared contract cases.
- BBO streams can fan out to both ticker and quote events when bid/ask sizes are available.
- Venue candle streams should be exposed as normalized external time bars instead of simulated by REST polling.

**Files:**
- Modify: `sdk/hyperliquid/perp/ws_market.go`
- Modify: `sdk/hyperliquid/perp/ws_method_test.go`
- Modify: `sdk/hyperliquid/spot/ws_market.go`
- Modify: `sdk/hyperliquid/spot/ws_method_test.go`
- Modify: `adapter/hyperliquid/adapter.go`
- Modify: `adapter/hyperliquid/helpers.go`
- Modify: `adapter/hyperliquid/perp.go`
- Modify: `adapter/hyperliquid/spot.go`
- Test: `adapter/hyperliquid/hyperliquid_test.go`

- [x] Confirm Hyperliquid's official WebSocket subscriptions include `bbo`, `trades`, and `candle`.
- [x] Write failing SDK tests for spot/perp candle subscribe and unsubscribe helpers.
- [x] Write failing adapter tests for spot/perp ticker/quote shared-stream fan-out, trade ticks, and 1m candle bars.
- [x] Add spot/perp SDK `SubscribeCandle` and `UnsubscribeCandle` helpers over the official `candle` subscription payload.
- [x] Add Hyperliquid logical topic tracking so ticker and quote share the BBO stream safely.
- [x] Map public trade streams to normalized `model.TradeTick` with trade IDs and aggressor side.
- [x] Map public candle streams to normalized external last-price bars with interval conversion.
- [x] Declare Hyperliquid spot/perp `TradeTicks`, `QuoteTicks`, and `Bars` only after contract coverage passes.
- [x] Run `go test ./sdk/hyperliquid/perp -run 'TestWebsocketClient_(Subscribe|Unsubscribe)Candle' -count=1`.
- [x] Run `go test ./sdk/hyperliquid/spot -run 'TestWebsocketClient_(Subscribe|Unsubscribe)Candle' -count=1`.
- [x] Run `go test ./adapter/hyperliquid -run 'Test(Spot|Perp)DataClientStreamsNautilusMarketDataTypes' -count=1`.
- [x] Run `go test ./adapter/hyperliquid -run 'Test(Spot|Perp)ClientsPassVenueContractSuite|Test(Spot|Perp)DataClientStreams(BboAndOrderBook|NautilusMarketDataTypes)' -count=1`.
- [x] Run `go test ./adapter/hyperliquid -count=1`.
- [x] Run `go test ./sdk/hyperliquid/... -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -race -count=1 ./adapter/hyperliquid ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 64: StandX Public Quote And Trade Stream Capability

**Nautilus reference:**
- Capability claims must match real venue stream behavior and shared contract coverage.
- When a venue price stream lacks sizes, quote ticks should be derived from top-of-book depth updates instead of emitting lossy price-only quotes.
- Public trade streams should normalize trade price, size, aggressor side, timestamp, and a deterministic trade ID.

**Files:**
- Modify: `adapter/standx/adapter.go`
- Modify: `adapter/standx/helpers.go`
- Test: `adapter/standx/standx_test.go`

- [x] Write failing StandX tests for orderbook/quote shared-stream fan-out and public trade ticks.
- [x] Add StandX logical topic tracking so orderbook and quote share the `depth_book` stream safely.
- [x] Map top-of-book depth updates to normalized `model.QuoteTick`.
- [x] Map `public_trade` payloads to normalized `model.TradeTick` with deterministic IDs and taker-side aggressor mapping.
- [x] Declare StandX `TradeTicks` and `QuoteTicks`; keep `Bars` unclaimed because the current SDK exposes no public candle WebSocket stream.
- [x] Run `go test ./adapter/standx -run TestDataClientStreamsNautilusMarketDataTypes -count=1`.
- [x] Run `go test ./adapter/standx -run 'TestPerpClientsPassVenueContractSuite|TestDataClientStreams(TickerAndOrderBook|NautilusMarketDataTypes)' -count=1`.
- [x] Run `go test ./adapter/standx -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -race -count=1 ./adapter/standx ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 65: Backpack Full Public Data Stream Capability

**Nautilus reference:**
- Public stream capability declarations must be backed by normalized events and shared contract tests.
- Quote ticks require size-aware top-of-book data; when book-ticker streams are price-only, derive quotes from the depth stream instead.
- Trade and bar streams should map venue payloads to normalized `TradeTick` and external time-bar events without falling back to REST polling.

**Files:**
- Modify: `adapter/backpack/adapter.go`
- Modify: `adapter/backpack/helpers.go`
- Test: `adapter/backpack/backpack_test.go`

- [x] Confirm Backpack's current public WebSocket stream names include `trade.<symbol>` and `kline.<interval>.<symbol>`.
- [x] Write failing Backpack tests for depth/quote shared-stream fan-out, public trade ticks, and 1m K-line bars.
- [x] Add Backpack logical stream tracking so orderbook and quote share the `depth.<symbol>` stream safely.
- [x] Map top-of-book depth updates to normalized `model.QuoteTick`.
- [x] Map public trade stream payloads to normalized `model.TradeTick` with aggressor side and trade IDs.
- [x] Map public K-line stream payloads to normalized external last-price bars with interval conversion.
- [x] Declare Backpack `TradeTicks`, `QuoteTicks`, and `Bars` only after contract coverage passes.
- [x] Run `go test ./adapter/backpack -run TestDataClientStreamsNautilusMarketDataTypes -count=1`.
- [x] Run `go test ./adapter/backpack -run 'TestPerpClientsPassVenueContractSuite|TestDataClientStreams(TickerAndOrderBook|NautilusMarketDataTypes)' -count=1`.
- [x] Run `go test ./adapter/backpack -count=1`.
- [x] Run `go test -count=1 ./testsuite ./adapter/... ./config/all`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite ./venue ./live`.
- [x] Run `go test -race -count=1 ./adapter/backpack ./testsuite ./platform ./backtest ./adapter/... ./config/all`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 19: Submit Order Lifecycle Closure

**Nautilus reference:**
- `OrderSubmitted` maps `Initialized/Released -> Submitted`.
- `OrderAccepted` maps `Submitted -> Accepted`.
- `OrderRejected` maps `Submitted -> Rejected`.
- Submit lifecycle events are command lifecycle events and must not create duplicate cached orders before the venue or simulator assigns the canonical order ID.

**Files:**
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`

- [x] Publish `OrderSubmitted` lifecycle after validation/risk succeeds and before live venue submission.
- [x] Publish `OrderAccepted` lifecycle after accepted venue reports are normalized and cached.
- [x] Publish `OrderRejected` lifecycle when live venue submission returns an error.
- [x] Publish backtest `OrderSubmitted` and `OrderAccepted` lifecycle events from the deterministic simulator acceptance path.
- [x] Keep lifecycle-only submit events out of the order cache to avoid duplicate temporary order IDs.
- [x] Update platform tests to wait for specific lifecycle kinds now that submit lifecycle events precede modify/cancel lifecycle events.
- [x] Run `go test ./platform -run 'TestNodeSubmitOrderPublishes' -count=1`.
- [x] Run `go test ./backtest -run TestBacktestSubmitOrderDispatchesSubmittedAndAcceptedLifecycle -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 20: Query Order Runtime Surface

**Nautilus reference:**
- `query_order(...)` is a first-class execution command available to strategies and routed to execution clients.
- Querying should be able to use local cache first and fall back to venue order status reporting without requiring a strategy to call reconciliation APIs directly.

**Files:**
- Modify: `model/order.go`
- Modify: `strategy/engine.go`
- Modify: `venue/interfaces.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `model/model_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`
- Test maintenance: `strategy/strategy_test.go`, `strategy/typed_test.go`

- [x] Add `model.QueryOrder` validation by account, instrument, and order/client/venue identity.
- [x] Add `Runtime.QueryOrder` for strategy-level Nautilus-style order inquiry.
- [x] Add optional `venue.OrderQuerier` for adapters with native single-order query APIs.
- [x] Implement platform query with cache-first lookup, optional native query, and fallback through `GenerateOrderStatusReports`.
- [x] Implement backtest query from deterministic cache state.
- [x] Run `go test ./model -run TestQueryOrderRequiresAccountInstrumentAndOrderIdentity -count=1`.
- [x] Run `go test ./platform -run TestNodeQueryOrderReturnsCachedOrderByClientID -count=1`.
- [x] Run `go test ./backtest -run TestBacktestQueryOrderReturnsCachedOrderToStrategy -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Note: one full non-SDK run saw an OKX swap subscribe EOF; `go test -count=1 ./adapter/okx -run TestSwapClientsPassVenueContractSuite` passed immediately after, and the final full non-SDK run passed.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `git diff --check`.

### Task 21: Query Account Runtime Surface

**Nautilus reference:**
- `QueryAccount` is a first-class execution command carrying an account ID.
- Execution clients handle `query_account` and emit/return current account state.
- Strategies should not need to call venue clients directly to refresh account state.

**Files:**
- Modify: `model/account.go`
- Modify: `strategy/engine.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `model/model_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`
- Test maintenance: `strategy/strategy_test.go`, `strategy/typed_test.go`

- [x] Add `model.QueryAccount` validation by account ID.
- [x] Add `Runtime.QueryAccount` for strategy-level account inquiry.
- [x] Implement platform account query by calling the execution client, applying the returned account snapshot to cache, and publishing an account execution event.
- [x] Implement deterministic backtest account query with a synthetic `BACKTEST` account snapshot when no cached snapshot exists.
- [x] Run `go test ./model -run TestQueryAccountRequiresAccountID -count=1`.
- [x] Run `go test ./platform -run TestNodeQueryAccountRefreshesCachesAndPublishesSnapshot -count=1`.
- [x] Run `go test ./backtest -run TestBacktestQueryAccountReturnsSnapshotToStrategy -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 22: Trade Tick Data Subscription And Backtest Closure

**Nautilus reference:**
- `TradeTick` is a first-class public market-data object with instrument, price, size, aggressor side, venue trade ID, event time, and init time.
- `SubscribeTradeTicks` / `UnsubscribeTradeTicks` are strategy-facing data commands routed through the data engine/client boundary.
- Trade ticks must dispatch to strategy callbacks, update shared cache, and participate in simulation matching instead of being a cosmetic payload type.

**Files:**
- Modify: `model/market_data.go`
- Modify: `cache/cache.go`
- Modify: `strategy/engine.go`
- Modify: `strategy/typed.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Modify: `testsuite/data_tester.go`
- Modify: adapter market-data default branches for unsupported subscription semantics.
- Test: `model/model_test.go`
- Test: `cache/cache_test.go`
- Test: `strategy/typed_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`
- Test: `testsuite/contracts_test.go`

- [x] Add `model.TradeTick`, `model.AggressorSide`, and `MarketDataTypeTradeTick`.
- [x] Extend `MarketEvent` validation and instrument identity routing for trade ticks.
- [x] Cache latest trade tick by instrument.
- [x] Add `Runtime.SubscribeTradeTicks` and `Runtime.UnsubscribeTradeTicks`.
- [x] Dispatch typed strategy `OnTradeTick(context.Context, model.TradeTick)` callbacks.
- [x] Implement platform trade tick subscribe/unsubscribe through the unified data subscription command.
- [x] Use trade ticks as a backtest matching price source for market, limit, triggered, trailing, and market-to-limit flows.
- [x] Add `TC-D13 Subscribe trade ticks` to the shared data contract suite.
- [x] Make adapters without trade tick streams return `ErrNotSupported` instead of silently accepting unknown supported-but-unimplemented market-data types.
- [x] Run `go test ./model -run 'TestTradeTick|TestSubscribeMarketData' -count=1`.
- [x] Run `go test ./cache -run TestCacheStoresLatestMarketEventsByInstrument -count=1`.
- [x] Run `go test ./strategy -run TestTypedStrategyDispatchesTradeTickCallbacks -count=1`.
- [x] Run `go test ./platform -run TestNodeSubscribesTradeTicksForwardsEventsAndCachesLatest -count=1`.
- [x] Run `go test ./backtest -run TestBacktestMatchesMarketOrderAgainstTradeTick -count=1`.
- [x] Run `go test ./testsuite -run TestDataTesterReportsNautilusStyleCaseResults -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 23: Bar Data Subscription And Backtest Closure

**Nautilus reference:**
- `BarType` identifies instrument, bar specification, and aggregation source.
- `Bar` is a first-class market-data object with OHLCV, event time, init time, and revision semantics.
- `SubscribeBars` / `UnsubscribeBars` are strategy-facing data commands routed through the data client boundary.
- Bar data must dispatch to strategy callbacks, update shared cache, and participate in simulation matching.

**Files:**
- Modify: `model/market_data.go`
- Modify: `cache/cache.go`
- Modify: `strategy/engine.go`
- Modify: `strategy/typed.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Modify: `testsuite/data_tester.go`
- Modify: selected adapter unsubscribe paths for explicit unsupported bar semantics.
- Test: `model/model_test.go`
- Test: `cache/cache_test.go`
- Test: `strategy/typed_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`
- Test: `testsuite/contracts_test.go`

- [x] Add `model.BarType`, `model.Bar`, and `MarketDataTypeBar` with validation.
- [x] Extend `SubscribeMarketData` with bar type identity and bar-specific subscription keys.
- [x] Extend `MarketEvent` validation and instrument identity routing for bars.
- [x] Cache latest bar by canonical bar type.
- [x] Add `Runtime.SubscribeBars` and `Runtime.UnsubscribeBars`.
- [x] Dispatch typed strategy `OnBar(context.Context, model.Bar)` callbacks.
- [x] Implement platform bar subscribe/unsubscribe through the unified data subscription command.
- [x] Use bar close as a backtest matching price source for market, limit, triggered, trailing, and market-to-limit flows.
- [x] Use latest scalar price selection across ticker, trade tick, and bar for trigger/trailing decisions.
- [x] Add `TC-D14 Subscribe bars` to the shared data contract suite.
- [x] Make direct adapter unsubscribe paths return `ErrNotSupported` for supported-but-unimplemented bar streams instead of building empty topics/channels.
- [x] Run `go test ./model -run 'TestBar|TestSubscribeMarketData' -count=1`.
- [x] Run `go test ./cache -run TestCacheStoresLatestMarketEventsByInstrument -count=1`.
- [x] Run `go test ./strategy -run TestTypedStrategyDispatchesBarCallbacks -count=1`.
- [x] Run `go test ./platform -run TestNodeSubscribesBarsForwardsEventsAndCachesLatest -count=1`.
- [x] Run `go test ./backtest -run TestBacktestMatchesMarketOrderAgainstBarClose -count=1`.
- [x] Run `go test ./testsuite -run TestDataTesterReportsNautilusStyleCaseResults -count=1`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.

### Task 24: Quote Tick Data Subscription And Backtest Closure

**Nautilus reference:**
- `QuoteTick` is a first-class top-of-book data object with bid/ask prices, bid/ask sizes, event time, and init time.
- `SubscribeQuoteTicks` / `UnsubscribeQuoteTicks` are strategy-facing data commands routed through the data client boundary.
- Quote ticks must dispatch to strategy callbacks, update shared cache, and participate in simulation matching with side-aware bid/ask prices and sizes.

**Files:**
- Modify: `model/market_data.go`
- Modify: `cache/cache.go`
- Modify: `strategy/engine.go`
- Modify: `strategy/typed.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Modify: `testsuite/data_tester.go`
- Modify: adapter subscribe/unsubscribe paths for pre-network unsupported quote/bar semantics.
- Test: `model/model_test.go`
- Test: `cache/cache_test.go`
- Test: `strategy/typed_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`
- Test: `testsuite/contracts_test.go`

- [x] Add `model.QuoteTick` and `MarketDataTypeQuoteTick` with crossed-quote and positive-size validation.
- [x] Extend `MarketEvent` validation and instrument identity routing for quote ticks.
- [x] Cache latest quote tick by instrument.
- [x] Add `Runtime.SubscribeQuoteTicks` and `Runtime.UnsubscribeQuoteTicks`.
- [x] Dispatch typed strategy `OnQuoteTick(context.Context, model.QuoteTick)` callbacks.
- [x] Implement platform quote tick subscribe/unsubscribe through the unified data subscription command.
- [x] Use quote ticks as side-aware top-of-book backtest matching input with bid/ask size consumption.
- [x] Include quote side prices in latest trigger/trailing price selection.
- [x] Add `TC-D15 Subscribe quote ticks` to the shared data contract suite.
- [x] Move unsupported market-data type checks before adapter provider/ws/network work to avoid false live subscriptions for quote/bar/trade streams.
- [x] Run `go test ./model -run 'TestQuoteTick|TestSubscribeMarketData' -count=1`.
- [x] Run `go test ./cache -run TestCacheStoresLatestMarketEventsByInstrument -count=1`.
- [x] Run `go test ./strategy -run TestTypedStrategyDispatchesQuoteTickCallbacks -count=1`.
- [x] Run `go test ./platform -run TestNodeSubscribesQuoteTicksForwardsEventsAndCachesLatest -count=1`.
- [x] Run `go test ./backtest -run TestBacktestMatchesMarketOrderAgainstQuoteTick -count=1`.
- [x] Run `go test ./testsuite -run TestDataTesterReportsNautilusStyleCaseResults -count=1`.
- [x] Run `go test -count=1 ./adapter/...`.
- [x] Run `go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite`.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run `go test -race -count=1 ./account ./platform ./backtest ./testsuite`.
- [x] Run `go test -run '^$' ./sdk/...`.
- [x] Run `git diff --check`.
