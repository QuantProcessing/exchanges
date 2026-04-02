# Complete Watch Fills Rollout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `WatchFills` for every exchange adapter in this repository where the exchange already exposes private execution data, while keeping `WatchOrders` lifecycle-only and adapter behavior honest when native fill data is missing.

**Architecture:** Reuse three implementation paths instead of inventing one-off logic per adapter: map execution-only callbacks out of existing order-update feeds where the exchange pushes fill events inline; wire native private fill/trade streams where the SDK already supports them; and add thin sdk subscription helpers only where the wire protocol already carries private fills but the repository has not exposed the subscription yet. Unsupported cases should only remain where the exchange-native stream is genuinely absent or too incomplete to model as `Fill`.

**Tech Stack:** Go, repository root streaming contracts in `exchange.go` and `models.go`, per-exchange adapters, low-level `sdk/*` websocket clients, `stretchr/testify`, repository unsupported-path tests.

---

### Task 1: Finish Exchange Capability Inventory

**Files:**
- Modify: `docs/superpowers/plans/2026-04-02-complete-watch-fills-rollout.md`
- Inspect: `aster/*`, `backpack/*`, `binance/*`, `bitget/*`, `decibel/*`, `edgex/*`, `lighter/*`, `okx/*`, `standx/*`

- [ ] Confirm per-exchange implementation path:
  - Inline execution events from order streams: `aster`, `binance`, `backpack`, `okx`
  - Native private fill/trade stream already in sdk: `lighter`, `standx`
  - Native fill data in account feed but sdk helper missing: `edgex`, `decibel`
  - Separate private fill channel available but repository helper missing: `bitget`
- [ ] Record any exchange that still cannot honestly support `WatchFills` after inspection.

### Task 2: Inline Fill Extraction From Existing Order Streams

**Files:**
- Modify: `aster/perp_adapter.go`
- Modify: `aster/spot_adapter.go`
- Modify: `binance/perp_adapter.go`
- Modify: `binance/spot_adapter.go`
- Modify: `backpack/streams.go`
- Modify: `backpack/private_mapping.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`
- Create: exchange-specific `fills_test.go` files where needed

- [ ] Add adapter-local mappers that emit `*exchanges.Fill` only when the order event is actually a fill/trade update.
- [ ] Use exchange-native fill identifiers and per-fill fields when present:
  - Binance/Aster spot: `TradeID`, `LastExecutedQuantity`, `LastExecutedPrice`, `CommissionAmount`, `CommissionAsset`, `IsMaker`
  - Binance/Aster perp: `TradeID`, `LastFilledQty`, `LastFilledPrice`, `Commission`, `CommissionAsset`, `IsMaker`
  - Backpack: `TradeID`, `FillQuantity`, `FillPrice`, `Fee`, `FeeSymbol`, `IsMaker`
  - OKX: `TradeId`, `FillSz`, `FillPx`, `Fee`, `Rebate/RebateCcy` when applicable
- [ ] Keep `WatchOrders` unchanged except for any helper reuse.
- [ ] Add targeted tests proving non-fill order events do not emit fills and fill events do.

### Task 3: Native Private Trade Streams Already Exposed In SDK

**Files:**
- Modify: `lighter/perp_adapter.go`
- Modify: `lighter/spot_adapter.go`
- Modify: `standx/perp_adapter.go`
- Create: `lighter/fills_test.go`
- Create: `standx/fills_test.go`

- [ ] Wire `lighter` `account_all_trades/*` into `WatchFills`.
- [ ] Wire `standx` private `trade` websocket channel into `WatchFills`.
- [ ] Map symbol, side, order id, fee asset, fee amount, maker/taker role, and timestamp using existing adapter helpers.
- [ ] Add focused mapping tests for one perp and one spot lighter trade payload, plus one StandX trade payload.

### Task 4: Add Missing SDK Subscription Helpers Where Private Fill Data Already Exists

**Files:**
- Modify: `decibel/sdk/ws/client.go`
- Modify: `decibel/sdk/ws/types.go`
- Modify: `decibel/perp_adapter.go`
- Modify: `decibel/perp_adapter_test.go`
- Modify: `edgex/sdk/perp/ws_account.go`
- Modify: `edgex/perp_adapter.go`
- Create: `edgex/fills_test.go`

- [ ] Add `decibel` `user_trades:{userAddr}` subscription support to the ws client and types.
- [ ] Map `decibel` trade pushes directly into `Fill`.
- [ ] Add an `EdgeX` account subscription helper for `ORDER_FILL_FEE_INCOME`.
- [ ] Map `OrderFillTransaction` into `Fill`.
- [ ] Extend existing decibel adapter test doubles so fill subscriptions can be exercised without live traffic.

### Task 5: Add Bitget Classic Fill Channel Support

**Files:**
- Modify: `bitget/sdk/classic_types.go`
- Modify: `bitget/private_classic.go`
- Create: `bitget/fills_test.go`

- [ ] Add websocket payload structs and decoders for Bitget classic private `fill` channel for spot and perp.
- [ ] Subscribe/unsubscribe `classicPerpProfile` and `classicSpotProfile` to `channel: fill`.
- [ ] Map exchange-native fill records into `Fill`.
- [ ] Preserve profile forwarding in `bitget/perp_adapter.go` and `bitget/spot_adapter.go`.

### Task 6: Documentation And Compatibility Tests

**Files:**
- Modify: `README.md`
- Modify: `README_CN.md`
- Modify: unsupported-path tests for any adapters that remain unsupported

- [ ] Update support guidance so examples mention that many more adapters now support `WatchFills`.
- [ ] Keep any truly unsupported adapters explicit in tests with `ErrNotSupported`.
- [ ] Verify README examples still reflect the lifecycle-vs-execution boundary.

### Task 7: Verification

**Files:**
- Inspect only

- [ ] Run focused tests for each exchange group as implementation lands.
- [ ] Run root model tests:

```bash
env GOCACHE=/tmp/go-build go test . -run 'Test(OrderSupportsExplicitOrderAndFillPrices|FillCarriesExecutionDetails|ExplicitOrderPriceFieldsCanCoexistWithLegacyPrice)$' -count=1
```

- [ ] Run targeted exchange tests for all new fill mappings.
- [ ] Run full compile sweep:

```bash
env GOCACHE=/tmp/go-build go test ./... -run '^$' -count=1
```
