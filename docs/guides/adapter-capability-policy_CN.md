# Adapter 能力策略

Adapter capabilities 是 claims，不是 aspirations。只有 adapter 实现对应接口并通过
shared contract tests 时，能力才可以标记为 `Yes`。缺失能力必须表达为 unsupported
behavior 或 `Planned`，不能用 no-op success 隐藏。

## Capability Matrix

使用 [Adapter 能力矩阵](../parity/adapter-capability-matrix_CN.md) 查看当前支持矩阵。
它区分：

- data snapshots；
- data streams；
- account snapshots；
- submit、cancel、modify、query；
- order、fill、position reports；
- private stream；
- resubscribe；
- mass status；
- order lists。

[Adapter Live Test Policy](../parity/adapter-live-test-policy_CN.md) 定义 public reads、
private reads 和 mutating live write tests 的测试边界。

private stream 与 resubscribe 是不同 lifecycle claims。adapter 可以支持 private
execution events，但尚未证明 restart resubscription；也可以支持 resubscription，
但不声明 mass-status recovery。

## Contract Rule

`testsuite.AdapterCapabilitySuite` 校验 `venue.DeclaredCapabilities` 与 optional
interfaces 的映射，例如 `venue.ExecutionResubscriber`、
`venue.ExecutionMassStatusGenerator` 和 `venue.OrderListSubmitter`。

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... ./config/all -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe' -v
```

## Live Test Gates

Read-only SDK/adapter tests 在不 mutate exchange state 时可以默认运行。live write test
必须同时由 credentials 与 exchange-specific enable flag gate，例如
`BYBIT_ENABLE_WS_ORDER_TESTS=1`。

live write 指任何可能 place、cancel、modify、transfer 或 mutate venue state 的测试。
除非 operator 显式 opt-in，否则必须清晰 skip。

## Release Rule

Release notes 必须列出：

- 当前计为 supported 的 adapters；
- 仍缺失的 planned capabilities；
- 当前 SDK scope 外的 extension targets；
- 用作证据的 exact contract command。
