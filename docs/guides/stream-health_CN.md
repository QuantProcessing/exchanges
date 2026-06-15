# Stream Health

`account.TradingAccount.Health()` 是 account runtime 的同步 health snapshot。它足够小，
应用可以轮询它，而不需要订阅每一个 execution event。

## Fields

- `Ready`：trading account 已连接、加载 account state，并完成 startup reconciliation。
- `AccountReady`：初始 account snapshot 已加载。
- `OrderStreamReady`：private execution event stream 可用，且当前 stream 未失败关闭。
- `FillsUnsupported`：execution client 不支持 fill report generation。
- `PositionsUnsupported`：execution client 不支持 position report generation。
- `Reconnects`：event channel closed 后完成的 private-stream recoveries 数。
- `Reconciliations`：startup 后完成的 periodic gap-reconciliation passes 数。
- `AccountEvents`、`OrderEvents`、`FillEvents`、`PositionEvents`：startup 后已应用的
  normalized execution events。
- `SlowSubscriberDrops`：per-order tracker channel 满时丢弃的 tracker updates 数。
- `LastEventTime`：最近 applied event 的 wall-clock time。
- `LastReconcileTime`：最近 successful periodic reconciliation 的 wall-clock time。
- `LastError`：最近 connect、reconciliation、stream 或 event-application error。

## Semantics

startup reconciliation 会在 `Ready` 变 true 前加载 account state、open order reports、
optional fill reports 和 optional position reports。

private execution stream 关闭时，`TradingAccount` 会 reconnect execution client，支持时调用
`venue.ExecutionResubscriber`，运行 gap reconciliation，然后从新的 event channel 继续。
如果 client 不能提供新的 stream，`OrderStreamReady` 保持 false，`LastError` 记录失败。

`OrderTracker` 是 best-effort fan-out consumer。即使 public channel 满了，它仍保留最新
order snapshot，并增加 `SlowSubscriberDrops`。

设置 `TradingAccountConfig.ReconcileInterval` 可启用 periodic account/order/fill/position
reconciliation。
