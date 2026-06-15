package testsuite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type CacheTesterConfig struct {
	Cache *cache.Cache
}

type CacheTester struct {
	cfg CacheTesterConfig
}

func NewCacheTester(cfg CacheTesterConfig) *CacheTester {
	if cfg.Cache == nil {
		cfg.Cache = cache.New()
	}
	return &CacheTester{cfg: cfg}
}

func (c *CacheTester) Run(_ context.Context, t *testing.T) ContractReport {
	t.Helper()
	return runContractCases(t, "cache", []contractCase{
		{id: "TC-C01", name: "Order runtime indexes", run: func() error {
			store := cache.New()
			order := cacheTestOrder()
			if err := store.PutOrder(order); err != nil {
				return err
			}
			if len(store.OrdersByStrategy(order.AccountID, order.Metadata.StrategyID)) != 1 {
				return errCase("missing strategy order index")
			}
			if len(store.OrdersByPositionID(order.AccountID, order.PositionID)) != 1 {
				return errCase("missing position order index")
			}
			if len(store.OrdersByOrderListID(order.AccountID, order.OrderListID)) != 1 {
				return errCase("missing order-list index")
			}
			if len(store.OrdersByExecSpawnID(order.AccountID, order.Metadata.ExecSpawnID)) != 1 {
				return errCase("missing exec-spawn index")
			}
			return nil
		}},
		{id: "TC-C02", name: "Fill runtime indexes", run: func() error {
			store := cache.New()
			fill := cacheTestFill()
			stored, err := store.PutFill(fill)
			if err != nil {
				return err
			}
			if !stored {
				return errCase("first fill should be stored")
			}
			stored, err = store.PutFill(fill)
			if err != nil {
				return err
			}
			if stored {
				return errCase("duplicate fill should not be stored")
			}
			if _, ok := store.FillByTradeID(fill.AccountID, fill.TradeID); !ok {
				return errCase("missing trade-id fill index")
			}
			if len(store.FillsByVenueOrderID(fill.AccountID, fill.VenueOrderID)) != 1 {
				return errCase("missing venue-order fill index")
			}
			return nil
		}},
		{id: "TC-C03", name: "Deferred fill storage", run: func() error {
			store := cache.New()
			fill := cacheTestFill()
			stored, err := store.PutDeferredFill(fill)
			if err != nil {
				return err
			}
			if !stored || len(store.DeferredFillsForOrder(fill.AccountID, fill.OrderID)) != 1 {
				return errCase("missing deferred fill")
			}
			if store.Residuals(fill.AccountID).DeferredFills != 1 {
				return errCase("deferred fill residual not counted")
			}
			store.ClearDeferredFillsForOrder(fill.AccountID, fill.OrderID)
			if len(store.DeferredFillsForOrder(fill.AccountID, fill.OrderID)) != 0 {
				return errCase("deferred fills not cleared")
			}
			return nil
		}},
		{id: "TC-C04", name: "Position runtime indexes", run: func() error {
			store := cache.New()
			position := cacheTestPosition()
			if err := store.PutPosition(position); err != nil {
				return err
			}
			if _, ok := store.PositionByVenueID(position.AccountID, position.VenuePositionID); !ok {
				return errCase("missing venue-position index")
			}
			if len(store.PositionsByStrategy(position.AccountID, position.Metadata.StrategyID)) != 1 {
				return errCase("missing strategy position index")
			}
			if len(store.OpenPositions(position.AccountID)) != 1 {
				return errCase("missing open position")
			}
			position.Side = model.PositionSideFlat
			position.Quantity = decimal.Zero
			if err := store.PutPosition(position); err != nil {
				return err
			}
			if len(store.ClosedPositions(position.AccountID)) != 1 {
				return errCase("missing closed position")
			}
			return nil
		}},
		{id: "TC-C05", name: "Account snapshot history", run: func() error {
			store := cache.New()
			first := model.AccountSnapshot{AccountID: "acct", Venue: "BINANCE", Type: model.AccountTypeMargin, Timestamp: time.Unix(100, 0)}
			second := first
			second.Timestamp = time.Unix(101, 0)
			store.PutAccount(first)
			store.PutAccount(second)
			if len(store.AccountHistory(first.AccountID)) != 2 {
				return errCase("missing account history")
			}
			account, ok := store.Account(first.AccountID)
			if !ok || !account.Timestamp.Equal(second.Timestamp) {
				return errCase("latest account snapshot not stored")
			}
			return nil
		}},
		{id: "TC-C06", name: "Market data snapshots", run: func() error {
			store := cache.New()
			instID := cacheTestInstrumentID()
			custom := model.CustomData{InstrumentID: instID, Type: "funding_rate", Fields: map[string]string{"rate": "0.0001"}}
			if err := store.PutMarketEvent(model.MarketEvent{Custom: &custom}); err != nil {
				return err
			}
			if _, ok := store.CustomData(instID, "funding_rate"); !ok {
				return errCase("missing custom data snapshot")
			}
			return nil
		}},
		{id: "TC-C07", name: "Residual summary", run: func() error {
			store := cache.New()
			order := cacheTestOrder()
			if err := store.PutOrder(order); err != nil {
				return err
			}
			position := cacheTestPosition()
			if err := store.PutPosition(position); err != nil {
				return err
			}
			if _, err := store.PutDeferredFill(cacheTestFill()); err != nil {
				return err
			}
			residuals := store.Residuals("acct")
			if residuals.OpenOrders != 1 || residuals.OpenPositions != 1 || residuals.DeferredFills != 1 {
				return errCase("unexpected residual counts")
			}
			return nil
		}},
		{id: "TC-C08", name: "Snapshot and purge", run: func() error {
			store := cache.New()
			account := model.AccountSnapshot{AccountID: "acct", Venue: "BINANCE", Type: model.AccountTypeMargin, Timestamp: time.Unix(100, 0)}
			store.PutAccount(account)
			nextAccount := account
			nextAccount.Timestamp = time.Unix(101, 0)
			store.PutAccount(nextAccount)
			closed := cacheTestOrder()
			closed.Status = model.OrderStatusFilled
			closed.FilledQuantity = closed.Quantity
			closed.LeavesQuantity = decimal.Zero
			closed.LastUpdatedTime = time.Unix(100, 0)
			if err := store.PutOrder(closed); err != nil {
				return err
			}
			open := cacheTestOrder()
			open.OrderID = "order-open"
			open.ClientOrderID = "client-open"
			open.VenueOrderID = "venue-open"
			open.LastUpdatedTime = time.Unix(101, 0)
			if err := store.PutOrder(open); err != nil {
				return err
			}
			snapshot := store.Snapshot("acct")
			if len(snapshot.AccountHistory) != 2 || len(snapshot.ClosedOrders) != 1 || len(snapshot.OpenOrders) != 1 {
				return errCase("unexpected cache snapshot")
			}
			result := store.Purge("acct", cache.PurgePolicy{ClosedOrdersLimit: 0, ClosedPositionsLimit: -1, AccountSnapshotsLimit: 1})
			if result.ClosedOrders != 1 || result.AccountSnapshots != 1 {
				return errCase("unexpected purge result")
			}
			if len(store.ClosedOrders("acct")) != 0 || len(store.OpenOrders("acct")) != 1 || len(store.AccountHistory("acct")) != 1 {
				return errCase("purge did not retain expected runtime state")
			}
			return nil
		}},
	})
}

func cacheTestInstrumentID() model.InstrumentID {
	return model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
}

func cacheTestOrder() model.OrderStatusReport {
	return model.OrderStatusReport{
		Metadata: model.CommandMetadata{
			StrategyID:  "strategy-001",
			ExecSpawnID: "spawn-001",
		},
		AccountID:      "acct",
		InstrumentID:   cacheTestInstrumentID(),
		OrderListID:    "list-001",
		PositionID:     "position-001",
		OrderID:        "order-001",
		VenueOrderID:   "venue-order-001",
		ClientOrderID:  "client-order-001",
		Status:         model.OrderStatusAccepted,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
	}
}

func cacheTestFill() model.FillReport {
	return model.FillReport{
		AccountID:     "acct",
		InstrumentID:  cacheTestInstrumentID(),
		OrderID:       "order-001",
		VenueOrderID:  "venue-order-001",
		ClientOrderID: "client-order-001",
		TradeID:       "trade-001",
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("0.4"),
		Timestamp:     time.Unix(100, 0),
	}
}

func cacheTestPosition() model.PositionStatusReport {
	return model.PositionStatusReport{
		Metadata:        model.CommandMetadata{StrategyID: "strategy-001"},
		AccountID:       "acct",
		InstrumentID:    cacheTestInstrumentID(),
		PositionID:      "position-001",
		VenuePositionID: "venue-position-001",
		Side:            model.PositionSideLong,
		Quantity:        decimal.RequireFromString("0.4"),
		EntryPrice:      decimal.RequireFromString("101"),
		Timestamp:       time.Unix(100, 0),
	}
}

func errCase(message string) error {
	return fmt.Errorf("%s", message)
}
