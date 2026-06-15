# 主评分卡

主评分卡是仓库面向 release 的质量契约。它把平台目标拆成 model、cache、command
metadata、strategy、data、execution、reconciliation、risk、portfolio、backtest、
live node、adapters 和 documentation 的可审计测试套件。

## Source Of Truth

- `testsuite` 定义 suite requirements、point weights、case IDs、package owners 和
  report sources。
- [完整功能矩阵](../parity/complete-feature-matrix_CN.md) 将 platform surfaces 映射到
  owners 和 acceptance suites。
- [Adapter 能力矩阵](../parity/adapter-capability-matrix_CN.md) 记录当前 venue
  capability claims。
- [完整质量门](../parity/complete-quality-gate_CN.json) 记录 release review 与
  verification requirements。

score 是 case-based。只有对应行为存在、经过测试且未 skipped 时，suite 才能获得 credit。

## 推荐命令

Focused metadata and scorecard checks：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Master|Score|Requirement|QualityGate|ReleaseNotes' -v
```

Runtime and lifecycle checks：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./account ./execution ./risk ./portfolio ./platform ./strategy ./backtest ./live ./testsuite -v
```

Adapter capability checks：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./adapter/... ./config/all ./testsuite -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe' -v
```

Hygiene checks：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -run '^$' -count=1 ./sdk/...
go vet ./...
git diff --check
```

## Release Rule

只有在相关 scorecard case passing、adapter declarations 与 evidence 匹配、
reconciliation unresolved discrepancy behavior 已记录、unsupported behavior 返回显式错误、
并附带 verification evidence 与 residual risks 时，release 才能声明对应 capability。
