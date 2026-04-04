package exchanges_test

import (
	"context"
	"errors"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTradingAccountPlaceReturnsFlowAndBackfillsOrderID(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
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

	adp := &accountRuntimeStubExchange{}
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

	acct := exchanges.NewTradingAccount(&accountRuntimeStubExchange{}, nil)
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

	adp := &accountRuntimeStubExchange{}
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

	adp := &accountRuntimeStubExchange{
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

func TestTradingAccountStartFailsWhenFetchAccountFails(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		fetchAccountErr: errors.New("boom"),
	}
	acct := exchanges.NewTradingAccount(adp, nil)

	err := acct.Start(context.Background())
	require.ErrorContains(t, err, "boom")
	require.Equal(t, int32(1), adp.fetchAccountCalls.Load())
	require.Zero(t, adp.watchOrdersCalls.Load())

	adp.fetchAccountErr = nil
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	require.Equal(t, int32(2), adp.fetchAccountCalls.Load())
	require.Equal(t, int32(1), adp.watchOrdersCalls.Load())
}

func TestTradingAccountStartFailsWhenWatchOrdersFails(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		watchOrdersErr: errors.New("watch-orders"),
	}
	acct := exchanges.NewTradingAccount(adp, nil)

	err := acct.Start(context.Background())
	require.ErrorContains(t, err, "watch-orders")
	require.Equal(t, int32(1), adp.fetchAccountCalls.Load())
	require.Equal(t, int32(1), adp.watchOrdersCalls.Load())

	adp.watchOrdersErr = nil
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	require.Equal(t, int32(2), adp.fetchAccountCalls.Load())
	require.Equal(t, int32(2), adp.watchOrdersCalls.Load())
}

func TestTradingAccountStartAllowsWatchPositionsUnsupported(t *testing.T) {
	t.Parallel()

	acct := exchanges.NewTradingAccount(&accountRuntimeStubExchange{
		watchPositionsErr: exchanges.ErrNotSupported,
	}, nil)

	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()
}

func TestTradingAccountPlaceWSCapturesSynchronousFirstUpdate(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		syncPlaceWSUpdates: []*exchanges.Order{{
			ClientOrderID: "ws-cli",
			OrderID:       "ws-exch",
			Status:        exchanges.OrderStatusNew,
		}},
	}
	acct := exchanges.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	placeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	flow, err := acct.PlaceWS(placeCtx, &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Price:    decimal.RequireFromString("100"),
		Quantity: decimal.RequireFromString("0.1"),
		ClientID: "ws-cli",
	})
	require.NoError(t, err)
	defer flow.Close()

	got, err := flow.Wait(context.Background(), func(o *exchanges.Order) bool {
		return o.OrderID == "ws-exch"
	})
	require.NoError(t, err)
	require.Equal(t, "ws-exch", got.OrderID)
}

func TestTradingAccountAppliesOrderAndPositionCaches(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{}
	acct := exchanges.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	adp.EmitOrder(&exchanges.Order{OrderID: "ord-1", Status: exchanges.OrderStatusNew})
	adp.EmitPosition(&exchanges.Position{Symbol: "ETH", Quantity: decimal.RequireFromString("1")})

	require.Eventually(t, func() bool {
		order, ok := acct.OpenOrder("ord-1")
		if !ok {
			return false
		}
		pos, ok := acct.Position("ETH")
		return ok && order.Status == exchanges.OrderStatusNew && pos.Quantity.Equal(decimal.RequireFromString("1"))
	}, time.Second, 10*time.Millisecond)
}
