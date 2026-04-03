package exchanges_test

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTradingAccountPlaceReturnsFlowAndBackfillsOrderID(t *testing.T) {
	t.Parallel()

	adp := &localStateStubExchange{
		placeResp: &exchanges.Order{
			ClientOrderID: "cli-1",
			Symbol:        "ETH",
			Side:          exchanges.OrderSideBuy,
			Type:          exchanges.OrderTypeLimit,
			Quantity:      decimal.RequireFromString("0.1"),
			Price:         decimal.RequireFromString("100"),
			Status:        exchanges.OrderStatusPending,
		},
		updates: []*exchanges.Order{{
			OrderID:       "exch-1",
			ClientOrderID: "cli-1",
			Symbol:        "ETH",
			Status:        exchanges.OrderStatusNew,
		}},
	}

	acct := exchanges.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	flow, err := acct.Place(context.Background(), &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Quantity: decimal.RequireFromString("0.1"),
		Price:    decimal.RequireFromString("100"),
	})
	require.NoError(t, err)
	defer flow.Close()

	require.Eventually(t, func() bool {
		latest := flow.Latest()
		return latest != nil && latest.OrderID == "exch-1"
	}, time.Second, 10*time.Millisecond)
}

func TestTradingAccountTrackRoutesUpdatesByClientIDAndOrderID(t *testing.T) {
	t.Parallel()

	adp := &localStateStubExchange{}
	acct := exchanges.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	flow, err := acct.Track("", "track-cli")
	require.NoError(t, err)
	defer flow.Close()

	adp.EmitOrder(&exchanges.Order{
		ClientOrderID: "track-cli",
		OrderID:       "track-exch",
		Status:        exchanges.OrderStatusNew,
	})
	adp.EmitOrder(&exchanges.Order{
		ClientOrderID:  "track-cli",
		OrderID:        "track-exch",
		Status:         exchanges.OrderStatusFilled,
		FilledQuantity: decimal.RequireFromString("0.2"),
	})

	got, err := flow.Wait(context.Background(), func(o *exchanges.Order) bool {
		return o.Status == exchanges.OrderStatusFilled
	})
	require.NoError(t, err)
	require.Equal(t, "track-exch", got.OrderID)
	require.Equal(t, decimal.RequireFromString("0.2"), got.FilledQuantity)
}
