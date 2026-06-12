# Repository Agent Notes

## Project Architecture Contract

This repository is organized around three explicit layers. All architecture,
feature, SDK, adapter, and TradingAccount work must preserve these boundaries.

### 1. SDK Layer: Native Exchange Capability

The exchange-local `sdk/` packages are the native protocol layer. They should
track official exchange spot/perp API documentation as closely as practical.

- SDK methods may expose venue-specific request/response types, endpoint names,
  and product concepts.
- SDK coverage should be audited against official documentation before broad
  implementation work. Use or update the official API parity documents under
  `docs/superpowers/gaps/` when SDK coverage changes.
- SDK testing must stay simple and method-local: every SDK source file should
  have a corresponding `_test.go` file, and every public API method should have
  a directly named test method. For example, `order.go` should have
  `order_test.go`, and `PlaceOrder` should have `TestClient_PlaceOrder`.
- SDK additions do not imply adapter additions. A venue-specific official API
  belongs in `sdk/` first; adapter exposure requires a separate abstraction
  decision.

### 2. Adapter Layer: Stable Convenience Abstraction

Adapters are not a mirror of every official API endpoint. They are a
standardized facade over the SDK for common trading workflows.

- Adapter responsibilities are symbol/instrument resolution, exchange-native to
  unified model mapping, order validation, common error mapping, and stable
  REST/WS convenience methods.
- Do not add venue-specific official API features directly to the core
  `Exchange`, `PerpExchange`, or `SpotExchange` interfaces just because the SDK
  supports them.
- Prefer small optional capability interfaces over growing the core interfaces.
  Examples: funding/open-interest market data, batch orders, algo/trigger
  orders, account bills, transfers, and venue-specific risk controls should be
  opt-in surfaces unless they become a proven cross-exchange primitive.
- Funding rate, funding history, open interest, mark/index analytics, and
  similar market-data features may be exposed by adapters, but they must not be
  treated as order lifecycle requirements.
- Base-symbol-only methods such as `FetchTicker(ctx, "BTC")` are legacy
  convenience, not the long-term adapter contract. New architecture work should
  prefer quote-aware or instrument-aware routing so users can address markets
  such as BTC/USDT and BTC/USDC through one logical adapter when the venue
  supports them.

### 3. TradingAccount Layer: Lifecycle Runtime

The public `account/` package owns high-level trading lifecycle behavior:
snapshots, streams, normalized order/fill reports, local order state,
balances/positions, and `OrderTracker`.

- TradingAccount should depend only on lifecycle-critical adapter capabilities:
  account snapshot, order placement/cancel/query, order reports, optional fill
  reports, and market-specific balance/position reports.
- TradingAccount must not require funding, open interest, historical market
  analytics, or venue-specific admin APIs.
- The new account readiness gate is startup reconciliation over
  `venue.ExecutionClient`. Legacy root adapter lifecycle support still requires
  a real `WatchOrders`.
- Multi-exchange, multi-quote, or portfolio-level lifecycle management should be
  built above individual TradingAccount instances rather than by overloading a
  single adapter with unrelated state.

### Interface Evolution Rules

- Keep core interfaces small and stable. Additions to core interfaces require a
  clear cross-exchange use case and migration plan.
- Add optional interfaces for new capability families before wiring them into
  adapters or TradingAccount.
- Breaking changes are allowed when they materially improve the SDK / adapter /
  TradingAccount boundary. There are no downstream applications that require
  preserving the current base-symbol-only adapter API.
- Capability claims in `register.go` must reflect real implementation and test
  coverage. Do not claim lifecycle or TradingAccount readiness without a real
  private order stream.
- Prefer explicit `ErrNotSupported` over no-op success for unsupported adapter
  surfaces.
- When in doubt, put exchange-specific behavior in `sdk/`, stable convenience
  behavior in adapters, and stateful lifecycle behavior in `account/`.

### Testing Rules

- Default tests must be practical Go tests: colocated, method-named, and easy
  to review.
- For SDK code, use one test file per implementation file and at least one test
  per public API method.
- SDK read method tests should call the real official exchange endpoint by
  default. Public read tests require no feature flag; private read tests may
  skip only when required credentials are missing.
- SDK write method tests must never execute by default. They must require an
  exchange-specific enable flag such as `BINANCE_ENABLE_LIVE_WRITE_TESTS=1`
  plus required credentials, and should skip with a clear message otherwise.
- Do not use fake HTTP transports, fake WebSocket connections, or local
  `httptest.Server` listeners for SDK API-method tests unless the method is a
  pure parser, pure signing helper, or local dispatcher with no exchange API
  side effect.
- Shared adapter and TradingAccount suites remain useful, but they do not
  replace method-level SDK tests.

## Adding Exchange Adapters

When the task is to add a new exchange, expand an adapter's support level, or rework adapter capability claims, read `docs/contributing/adding-exchange-adapters.md` before touching code.

Treat that document as repository policy. It is project-specific guidance for this codebase, not a reusable personal/global skill.
