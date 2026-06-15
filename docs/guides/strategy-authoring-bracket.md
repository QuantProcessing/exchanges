# Strategy Authoring With Brackets

Bracket orders express a common lifecycle: enter a position, then release a
take-profit and stop-loss pair that closes the position. This repository models
that workflow as a `model.OrderList` created by `model.OrderFactory`.

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

`OrderFactory.Bracket` creates:

- one parent limit entry order;
- one stop-market child;
- one limit take-profit child;
- one shared `OrderListID`;
- child `ParentClientOrderID` references;
- OTO/OCO contingency metadata;
- reduce-only exit children.

## Runtime Semantics

1. The parent entry order is submitted first.
2. Exit children are held by the platform/execution path.
3. When the parent fills, children are released.
4. When one OCO child fills, the sibling is canceled.
5. Cache records order and fill state.
6. Portfolio records position, fee, and PnL changes.
7. Strategy callbacks receive order status, lifecycle, fill, and position
   events.

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

Bracket behavior crosses strategy, execution, backtest, cache, and portfolio
state. Use focused tests for the package you changed and a broader example or
runtime test when changing list lifecycle behavior.

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./execution ./backtest ./examples/... -run 'Bracket|OrderList|OCO|OTO' -v
```
