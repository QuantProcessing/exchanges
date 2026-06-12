# Adapter Exposure Policy

Accessed architecture contract: `AGENTS.md`, 2026-06-10.

SDK parity can add venue-native endpoints faster than adapters should expose
them. A newly covered official endpoint stays SDK-only unless it fits an
existing stable adapter capability or a new optional interface has a written
design note and tests.

Current decision for the first parity pass:

- Batch order endpoints stay SDK-only/raw because semantics differ across venues.
- Algo, trigger, TWAP, and plan-order endpoints stay SDK-only/raw until a shared order-policy model is designed.
- Account bills, closed PnL, historical positions, and transfer-like endpoints stay SDK-only/raw because they are not TradingAccount lifecycle dependencies.
- Funding and open interest remain optional adapter market-analytics capabilities where already implemented.

Rejected exposure in this pass:

- `BatchOrderExchange`: not added; quantity, partial-failure, idempotency, and response semantics differ by venue.
- `AlgoOrderExchange`: not added; trigger conditions and lifecycle states are not yet normalized.
- `AccountBillsExchange`: not added; useful, but not part of trading lifecycle readiness.
- `TransferExchange`: not added; account-type semantics differ too much to normalize safely in this pass.
