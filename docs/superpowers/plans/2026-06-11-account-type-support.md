# Account Type Support Implementation Plan

**Goal:** Align SDK and adapter account-context behavior with official spot/perp APIs while preserving the repository's SDK / adapter / TradingAccount boundaries.

**Architecture:** SDKs expose official account-mode and account-context APIs. Adapters only use account mode when it changes lifecycle routing or target account context. TradingAccount remains exchange-account-mode agnostic.

## Task Queue

### Phase 1: Runtime Safety For Lifecycle Account Context

- [x] Hyperliquid: expose `VaultAddress` in adapter options and registry config, then pass it to REST and WS SDK clients.
- [x] Bybit: implement `GET /v5/account/info` in SDK and use `unifiedMarginStatus` to reject unsupported classic account lifecycle mode in adapters.
- [x] Bitget: add explicit `AccountMode` option and fail fast when `UTA` is selected before UTA lifecycle profiles are implemented.

### Phase 2: Typed Account Metadata

- [x] OKX: add typed SDK account levels for `acctLv` and record account level in the perp adapter alongside position mode.
- [x] Lighter: add typed SDK account tiers for account limits.

### Phase 3: Binance Account Product Coverage

- [x] Audit official Binance spot, USD-M futures, margin, portfolio margin, and sub-account docs into `docs/superpowers/gaps/official-api-parity-binance.md`.
- [x] Keep margin / portfolio margin / sub-account APIs SDK-first unless a stable cross-exchange adapter capability is justified.
- [x] Add method-local live-read tests for Binance margin account APIs; write methods remain gated by `BINANCE_ENABLE_LIVE_WRITE_TESTS=1`.
- [x] Add first SDK-only portfolio margin account methods for `/papi/v1/balance` and `/papi/v1/account`.
- [x] Add sub-account asset management SDK methods as admin-only SDK surfaces.

### Phase 4: Bitget UTA SDK Coverage

- [x] Add SDK methods and method-local tests for official UTA assets, orders, positions, and private order/fill/position streams.
- [x] Add a UTA private profile beside the current classic profile.
- [x] Change adapter `AccountModeUTA` from fail-fast to real lifecycle support after REST order lifecycle and `WatchOrders` are implemented.
- [x] Add UTA WS place/cancel support and UTA `WatchFills` mapping so lifecycle surfaces align with classic capability claims.
- [x] Add SDK-only typed methods for first-pass non-lifecycle UTA account reads: account info, funding assets, financial records, fee rate, switch status, max transferable, and open-interest limit.
- [x] Add SDK-only write-gated method for UTA holding mode changes.
- [ ] Live-verify UTA private REST and private WS subscription success with a real UTA-enabled Bitget key.
- [ ] Add typed SDK methods for remaining non-lifecycle UTA account admin/transfer/loan/deduct/account-mode endpoints as SDK parity work continues.
- [ ] Update Bitget capability claims if future options make capabilities runtime-queryable; current static claims are valid for classic and now lifecycle-complete for implemented UTA order/fill surfaces, but live UTA verification remains credential-blocked.

### Phase 5: Adapter Optional Capabilities

- [ ] Add optional account-metadata capability interfaces only where repeated cross-exchange usage exists.
- [ ] Keep tier, VIP, permissions, sub-account admin, and portfolio-margin admin operations out of core `Exchange`, `SpotExchange`, `PerpExchange`, and TradingAccount.

## Verification Notes

- Public and private read SDK tests should call official endpoints directly when credentials are available.
- Private read tests may skip when credentials are missing.
- Live write tests must remain opt-in behind exchange-specific flags.
- Current live verification gap: Bybit `TestClient_GetAccountInfo` compiled but live network validation hit certificate/TLS failures against `https://api.bybit.com`.
- Current live verification gap: Binance signed margin, portfolio-margin, and sub-account live reads use server-time synced timestamps, but live responses still ended in `EOF` from the network/exchange edge.
- Current live verification gap: Bitget UTA private REST live tests skip with current classic-account credentials (`40084`), and UTA private WS login sometimes times out before subscription; public REST/WS and classic private REST live reads pass.
