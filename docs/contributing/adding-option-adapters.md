# Adding Option Adapters

Sibling rules to [`adding-exchange-adapters.md`](./adding-exchange-adapters.md).
Read that first; this doc only covers what's *different* for option markets.

## Before code

Three CEXes have public options APIs the library targets today:

- **Binance** (`eapi.binance.com`) — reference implementation, see `adapter/binance/option_adapter.go`
- **OKX** (`/api/v5/...` with `instType=OPTION`) — skeleton in `adapter/okx/option_adapter.go`
- **Bybit** (v5 `category=option`) — skeleton in `adapter/bybit/option_adapter.go`

CEXes without an options product (Bitget, Aster, Nado, Lighter, Hyperliquid, StandX, GRVT, EdgeX, Backpack) MUST NOT have option adapters — don't add stubs.

## Capability matrix

Pick a level explicitly and wire tests that match the claim. The old option
TradingAccount suite has been removed; option adapters currently need focused
adapter tests until an instrument-aware option venue suite exists.

| Claim                              | Required surface                                                                                                  | Required suites                                                  |
|------------------------------------|-------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------|
| `option-public-data-only`          | `FetchOptionChain` + `FetchExpirations` + `FetchOptionMark` + `FetchGreeks` + instrument format round-trip        | Focused adapter/unit tests for chain, marks, Greeks, and formatting |
| `option-trading-capable`           | + REST `PlaceOrder` / `CancelOrder` / `FetchOrderByID` / `FetchOpenOrders`                                        | + gated live passive place/cancel tests                          |
| `option-account-state-capable`     | + `FetchOptionPositions` and explicit stream support or `ErrNotSupported` for private option streams              | + focused position and unsupported-stream tests                  |

Surfaces a venue genuinely lacks MUST return `exchanges.ErrNotSupported` — never a silent no-op.

## Mandatory invariants

These invariants must be covered by focused adapter tests. Adapters failing them are broken:

1. **Instrument-ID round-trip**: `FormatInstrument(parsed) == FormatInstrument(orig)` for any chain entry. Use the typed `*OptionInstrument` everywhere internally; the wire string is for transport only.
2. **Underlying normalization**: `OptionInstrument.Underlying` is the base symbol (`"BTC"`), not the venue's spot pair (`"BTCUSDT"`). The adapter strips the quote suffix when parsing.
3. **Expiry resolution**: `OptionInstrument.Expiry` is a UTC `time.Time` at the venue's actual settlement instant (Binance uses 08:00 UTC; OKX/Bybit use 08:00 UTC; Deribit uses 08:00 UTC). Day-level round-trip is required by the suite; intraday agreement is recommended.
4. **`InstrumentType` discriminator**: every `Position` returned from `FetchOptionPositions` / `WatchPositions` MUST have `InstrumentType == InstrumentTypeOption` and `Option != nil`. The compliance test asserts both.
5. **Greeks sign convention**: long-call Δ > 0, long-put Δ < 0. `|Δ| ≤ 1`. If the venue publishes per-1% Vega/Theta, leave it; if per-unit, scale to per-1% in the adapter.
6. **Quantity unit**: `Position.Quantity` and `OptionOrderParams.Quantity` are in **contracts**. The underlying exposure is `Quantity × OptionPositionData.ContractSize`. Don't multiply in `Quantity`.
7. **Price unit**: `OptionOrderParams.Price` is the premium per contract in the contract's settlement currency.
8. **Settlement asset**: populate `OptionInstrument.Settlement` (`"USDT"` / `"USDC"` / `"USD"`). The wire instrument-ID may omit it; the typed struct must include it.

## Architecture rules (deltas from perp)

- SDK layer (`sdk/<venue>/option/`): owns signing, wire structs, REST/WS lifecycle. Never bleed wire-format structs into the adapter return surface.
- Adapter file (`adapter/<venue>/option_adapter.go`): implements `exchanges.OptionExchange` + the relevant `Exchange` / `Streamable` methods. For surfaces that don't fit option semantics (e.g. `FetchTicker` with a base symbol), return `ErrNotSupported`.
- Register only fully-implemented adapters in `adapter/<venue>/register.go`. Skeleton adapters returning `ErrNotSupported` for everything stay **unregistered** to prevent accidental wiring.
- `GetMarketType()` returns `exchanges.MarketTypeOption`.

## Symbol semantics (important)

The base library convention is "Exchange methods take a base currency". For options, this is **overloaded**: in `OptionExchange` methods, the string symbol is the **instrument ID** (`"BTC-251226-100000-C"`), not the underlying. Reasons:

- One underlying has thousands of instruments; per-underlying ticker / orderbook methods don't make sense.
- Order routing is per-instrument, so `OrderParams.Symbol` already needs to be the instrument ID.

`FormatSymbol` / `ExtractSymbol` are pass-through on option adapters.

## Live test wiring

`.env.example` additions:

```
BINANCE_OPTION_API_KEY=
BINANCE_OPTION_SECRET_KEY=
BINANCE_OPTION_TEST_UNDERLYING=BTC
BINANCE_OPTION_TEST_INSTRUMENT=
```

Option live tests must be explicitly gated by exchange-specific enable flags and credentials. Keep default CI/read-only coverage focused on public data, formatting, position mapping, and unsupported-surface behavior.

## Red flags

- Surfacing venue wire structs from adapter methods → SDK boundary leak.
- Returning a placeholder `Position` without `InstrumentType` set → compliance test fails.
- Multi-leg / combo orders → out of scope for v1. Multi-leg requires `MultiLegOrderParams` and is a separate PR.
- Re-using `Slippage` semantics on options → not defined; leave it as Perp-only.
- Calling adapter from SDK or vice-versa beyond the established direction.

## Reference files

- `adapter/binance/option_adapter.go` — fully working REST adapter
- `sdk/binance/option/` — wire structs, signing, REST client
- `adapter/binance/option_adapter_unit_test.go` — focused option adapter behavior tests
- `option_models.go` — `InstrumentType`, `OptionInstrument`, `Greeks`, `OptionPositionData`, `OptionMark`, `OptionChainOpts`
- `exchange.go` — `OptionExchange` interface declaration
