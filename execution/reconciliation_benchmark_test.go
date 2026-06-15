package execution

import (
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

func BenchmarkReconcilerMassStatus(b *testing.B) {
	accountID := model.AccountID("acct")
	instrumentID := executionTestInstrumentID()
	order := executionTestOrderReport("bench-order", "bench-client", model.OrderStatusAccepted)
	order.AccountID = accountID
	order.InstrumentID = instrumentID
	order.Quantity = decimal.RequireFromString("2")
	order.LeavesQuantity = decimal.RequireFromString("2")
	fill := model.FillReport{
		AccountID:     accountID,
		InstrumentID:  instrumentID,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		TradeID:       "bench-trade",
		PositionID:    "bench-position",
		Side:          model.OrderSideBuy,
		Quantity:      decimal.RequireFromString("0.75"),
		Price:         decimal.RequireFromString("100"),
		Fee:           decimal.RequireFromString("0.01"),
		FeeCurrency:   "USDT",
		Timestamp:     time.Unix(10, 0),
	}
	position := model.PositionStatusReport{
		AccountID:    accountID,
		InstrumentID: instrumentID,
		PositionID:   "bench-position",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("0.75"),
		EntryPrice:   decimal.RequireFromString("100"),
	}
	account := model.AccountSnapshot{
		AccountID: accountID,
		Venue:     "BINANCE",
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "999.99",
			Locked:   "0.01",
			Total:    "1000",
		}},
	}
	status := model.ExecutionMassStatus{
		AccountID: accountID,
		Venue:     "BINANCE",
		Accounts:  []model.AccountSnapshot{account},
		Orders:    []model.OrderStatusReport{order},
		Fills:     []model.FillReport{fill},
		Positions: []model.PositionStatusReport{position},
		Timestamp: time.Unix(11, 0),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reconciler := NewReconciler(ReconciliationConfig{Cache: cache.New()})
		if _, err := reconciler.ReconcileMassStatus(status); err != nil {
			b.Fatal(err)
		}
	}
}
