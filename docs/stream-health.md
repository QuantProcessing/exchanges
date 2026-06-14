# Stream Health

`account.TradingAccount.Health()` is the public account-runtime health snapshot.
It is intentionally small and synchronous so applications can poll it without
subscribing to every execution event.

## Fields

- `Ready`: the trading account has connected, loaded account state, and
  completed startup reconciliation.
- `AccountReady`: the initial account snapshot was loaded.
- `OrderStreamReady`: a private execution event stream is available and the
  current stream has not failed closed.
- `FillsUnsupported`: the execution client does not expose fill report
  generation for the configured instruments.
- `PositionsUnsupported`: the execution client does not expose position report
  generation for the configured instruments.
- `Reconnects`: number of private-stream recoveries completed after the event
  channel closed.
- `Reconciliations`: number of periodic gap-reconciliation passes completed
  after startup.
- `AccountEvents`, `OrderEvents`, `FillEvents`, `PositionEvents`: normalized
  execution events applied after startup.
- `SlowSubscriberDrops`: tracker updates dropped because a per-order tracker
  channel was full.
- `LastEventTime`: wall-clock time of the most recent applied event.
- `LastReconcileTime`: wall-clock time of the most recent successful periodic
  reconciliation pass.
- `LastError`: last connect, reconciliation, stream, or event-application
  error observed by the account runtime.

## Semantics

Startup reconciliation loads account state, open order reports, optional fill
reports, and optional position reports before `Ready` becomes true.

When a private execution stream closes, `TradingAccount` reconnects the
execution client, calls `venue.ExecutionResubscriber` when implemented, runs
gap reconciliation, then resumes forwarding from the fresh event channel. If
the client cannot provide a fresh stream, `OrderStreamReady` remains false and
`LastError` records the failure.

`OrderTracker` instances are best-effort fan-out consumers. They preserve the
latest order snapshot even when their public channel is full, and increment
`SlowSubscriberDrops` for dropped channel sends.

Set `TradingAccountConfig.ReconcileInterval` to enable periodic account, order,
fill, and position reconciliation. Missing local open orders are marked
`canceled`, and missing non-flat local positions are flattened when the
execution client supports position report generation.
