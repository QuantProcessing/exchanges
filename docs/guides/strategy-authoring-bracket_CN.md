# Bracket 策略编写

Bracket orders 表达一个常见生命周期：先进入仓位，再释放 take-profit 和 stop-loss
退出订单。本仓库用 `model.OrderFactory` 创建 `model.OrderList` 来建模这个流程。

## Bracket Shape

```go
list := rt.OrderFactory(accountID).Bracket(model.BracketOrderRequest{
    InstrumentID: instrumentID,
    Side:         model.OrderSideBuy,
    Quantity:     decimal.RequireFromString("1"),
    EntryPrice:   decimal.RequireFromString("100"),
    TakeProfit:   decimal.RequireFromString("104"),
    StopLoss:     decimal.RequireFromString("98"),
})

reports, err := rt.SubmitOrderList(ctx, list)
```

`OrderFactory.Bracket` 会创建：

- 一个 parent limit entry order；
- 一个 stop-market child；
- 一个 limit take-profit child；
- 一个共享 `OrderListID`；
- child `ParentClientOrderID` references；
- OTO/OCO contingency metadata；
- reduce-only exit children。

## Runtime Semantics

1. parent entry order 先提交；
2. exit children 被 platform/execution path hold；
3. parent fill 后 children release；
4. 一个 OCO child fill 后 sibling cancel；
5. cache 记录 order/fill state；
6. portfolio 记录 position、fee 和 PnL；
7. 策略 callbacks 收到 order status、lifecycle、fill 和 position events。

## Strategy Pattern

```go
func (s *BracketStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
    if s.submitted || book.InstrumentID != s.instrumentID || !s.entryTouched(book) {
        return nil
    }
    s.submitted = true

    list := s.runtime.OrderFactory(s.accountID).Bracket(model.BracketOrderRequest{
        InstrumentID: s.instrumentID,
        Side:         model.OrderSideBuy,
        Quantity:     decimal.RequireFromString("1"),
        EntryPrice:   decimal.RequireFromString("100"),
        TakeProfit:   decimal.RequireFromString("104"),
        StopLoss:     decimal.RequireFromString("98"),
    })
    _, err := s.runtime.SubmitOrderList(ctx, list)
    return err
}
```

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./execution ./backtest ./examples/... -run 'Bracket|OrderList|OCO|OTO' -v
```
