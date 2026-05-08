# Stream Health

`account.TradingAccount` exposes a copyable health snapshot for account-runtime
streams. This is intended for monitoring, alerts, and SDK-user preflight checks
after `Start`.

```go
health := acct.Health()
orders := health.Streams[account.StreamOrders]
if orders.DroppedEvents > 0 {
    // A subscriber was too slow and missed order events.
}
```

## Stream States

| Status | Meaning |
|--------|---------|
| `unknown` | The account runtime has not started a stream yet. |
| `starting` | `Start` is establishing the stream. |
| `ready` | The stream subscription call succeeded. |
| `unsupported` | The adapter returned `exchanges.ErrNotSupported`; startup may continue for optional streams. |
| `error` | A required stream failed or an optional stream failed with a non-unsupported error. |
| `stopped` | `TradingAccount.Close` stopped the runtime. |

## Fields

`TradingAccountHealth` reports whether the runtime is started, whether the
initial REST snapshot has loaded, when the latest snapshot was applied, and a
per-stream map.

Each `StreamHealth` contains:

- `Supported` and `Ready`
- total stream `Events`
- `DroppedEvents` caused by full account-runtime subscriber channels
- `LastEventAt`
- `LastError` and `LastErrorAt`

## Scope

This is runtime observability, not exchange-specific reconnect orchestration.
Adapters may already reconnect internally, but this contract only records what
the account runtime can know today: subscription startup, unsupported optional
streams, delivered events, and slow-subscriber drops.
