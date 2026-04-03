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

func TestTradingAccountCloseClosesTrackedFlows(t *testing.T) {
	t.Parallel()

	acct := exchanges.NewTradingAccount(&localStateStubExchange{}, nil)
	require.NoError(t, acct.Start(context.Background()))

	flow, err := acct.Track("", "close-cli")
	require.NoError(t, err)

	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan struct{})
	var waitErr error
	go func() {
		defer close(done)
		_, waitErr = flow.Wait(waitCtx, func(o *exchanges.Order) bool {
			return o.Status == exchanges.OrderStatusFilled
		})
	}()

	acct.Close()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected tracked flow wait to unblock when account closes")
	}
	require.EqualError(t, waitErr, "order flow closed")
}

func TestTradingAccountStartIsIdempotent(t *testing.T) {
	t.Parallel()

	adp := &localStateStubExchange{}
	acct := exchanges.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(context.Background()))
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	flow, err := acct.Track("", "dup-cli")
	require.NoError(t, err)
	defer flow.Close()

	adp.EmitOrder(&exchanges.Order{
		ClientOrderID: "dup-cli",
		OrderID:       "dup-order",
		Status:        exchanges.OrderStatusNew,
	})

	select {
	case update := <-flow.C():
		require.NotNil(t, update)
		require.Equal(t, "dup-order", update.OrderID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected first order update")
	}

	select {
	case update := <-flow.C():
		t.Fatalf("unexpected duplicate order update after repeated Start: %+v", update)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestTradingAccountPlaceCapturesSynchronousFirstUpdate(t *testing.T) {
	t.Parallel()

	adp := &localStateStubExchange{
		placeResp: &exchanges.Order{
			ClientOrderID: "sync-cli-1",
			Symbol:        "ETH",
			Side:          exchanges.OrderSideBuy,
			Type:          exchanges.OrderTypeLimit,
			Quantity:      decimal.RequireFromString("0.1"),
			Price:         decimal.RequireFromString("100"),
			Status:        exchanges.OrderStatusPending,
		},
		syncPlaceUpdates: []*exchanges.Order{{
			OrderID:       "sync-exch-1",
			ClientOrderID: "sync-cli-1",
			Symbol:        "ETH",
			Status:        exchanges.OrderStatusNew,
		}},
		placeReturnDelay: 25 * time.Millisecond,
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
		return latest != nil && latest.OrderID == "sync-exch-1" && latest.Status == exchanges.OrderStatusNew
	}, 200*time.Millisecond, 10*time.Millisecond)
}
