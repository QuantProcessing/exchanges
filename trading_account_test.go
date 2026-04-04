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

func TestTradingAccountCloseClosesPreStartTrackedFlowsAndSubscriptions(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		fetchAccountErr: errors.New("fetch-account"),
	}
	acct := exchanges.NewTradingAccount(adp, nil)

	flow, err := acct.Track("", "prestart-cli")
	require.NoError(t, err)
	orderSub := acct.SubscribeOrders()
	positionSub := acct.SubscribePositions()

	err = acct.Start(context.Background())
	require.ErrorContains(t, err, "fetch-account")

	acct.Close()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	waitErrCh := make(chan error, 1)
	go func() {
		_, waitErr := flow.Wait(waitCtx, func(*exchanges.Order) bool {
			return false
		})
		waitErrCh <- waitErr
	}()

	select {
	case waitErr := <-waitErrCh:
		require.EqualError(t, waitErr, "order flow closed")
	case <-time.After(time.Second):
		t.Fatal("expected pre-start tracked flow to close")
	}

	select {
	case _, ok := <-orderSub.C:
		require.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("expected pre-start order subscription to close")
	}

	select {
	case _, ok := <-positionSub.C:
		require.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("expected pre-start position subscription to close")
	}
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

func TestTradingAccountStartFailsWhenWatchPositionsFails(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		watchPositionsErr: errors.New("watch-positions"),
	}
	acct := exchanges.NewTradingAccount(adp, nil)

	err := acct.Start(context.Background())
	require.ErrorContains(t, err, "watch-positions")
	require.Equal(t, int32(1), adp.fetchAccountCalls.Load())
	require.Equal(t, int32(1), adp.watchOrdersCalls.Load())
	require.Equal(t, int32(1), adp.watchPositionsCalls.Load())

	adp.watchPositionsErr = nil
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	require.Equal(t, int32(2), adp.fetchAccountCalls.Load())
	require.Equal(t, int32(2), adp.watchOrdersCalls.Load())
	require.Equal(t, int32(2), adp.watchPositionsCalls.Load())
}

func TestTradingAccountStartCancelsWatchOrdersWhenWatchPositionsFails(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		watchPositionsErr: errors.New("watch-positions"),
		emitOrderOnCancel: &exchanges.Order{
			OrderID: "cancel-window-order",
			Status:  exchanges.OrderStatusNew,
		},
	}
	acct := exchanges.NewTradingAccount(adp, nil)
	orderSub := acct.SubscribeOrders()
	defer orderSub.Unsubscribe()

	err := acct.Start(context.Background())
	require.ErrorContains(t, err, "watch-positions")

	require.Eventually(t, func() bool {
		return adp.orderCancelEmits.Load() == 1 && adp.watchOrdersCanceled.Load() == 1
	}, time.Second, 10*time.Millisecond)

	require.Never(t, func() bool {
		select {
		case order, ok := <-orderSub.C:
			return ok && order != nil && order.OrderID == "cancel-window-order"
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond)
}

func TestTradingAccountStartCleansFailedSnapshotState(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		fetchAccountResp: &exchanges.Account{
			TotalBalance: decimal.RequireFromString("7"),
			Positions: []exchanges.Position{{
				Symbol:   "ETH",
				Quantity: decimal.RequireFromString("1"),
				Side:     exchanges.PositionSideLong,
			}},
			Orders: []exchanges.Order{{
				OrderID:  "stale-order",
				Symbol:   "ETH",
				Status:   exchanges.OrderStatusNew,
				Quantity: decimal.RequireFromString("0.1"),
			}},
		},
		watchOrdersErr: errors.New("watch-orders"),
	}
	acct := exchanges.NewTradingAccount(adp, nil)

	err := acct.Start(context.Background())
	require.ErrorContains(t, err, "watch-orders")

	adp.fetchAccountResp = &exchanges.Account{}
	adp.watchOrdersErr = nil
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	require.True(t, acct.Balance().IsZero())
	_, ok := acct.OpenOrder("stale-order")
	require.False(t, ok)
	_, ok = acct.Position("ETH")
	require.False(t, ok)
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

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()

	got, err := flow.Wait(waitCtx, func(o *exchanges.Order) bool {
		return o.OrderID == "ws-exch"
	})
	require.NoError(t, err)
	require.Equal(t, "ws-exch", got.OrderID)
}

func TestTradingAccountCloseCancelsBlockedStart(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		fetchAccountBlock:   make(chan struct{}),
		fetchAccountStarted: make(chan struct{}),
	}
	acct := exchanges.NewTradingAccount(adp, nil)

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- acct.Start(context.Background())
	}()

	select {
	case <-adp.fetchAccountStarted:
	case <-time.After(time.Second):
		t.Fatal("expected Start to reach FetchAccount")
	}

	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		acct.Close()
	}()

	select {
	case err := <-startErrCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("expected Start to abort when Close cancels blocked FetchAccount")
	}

	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("expected Close to complete without manually releasing FetchAccount")
	}

	require.Equal(t, int32(1), adp.fetchAccountCalls.Load())
	require.Zero(t, adp.watchOrdersCalls.Load())
	require.Zero(t, adp.watchPositionsCalls.Load())
}

func TestTradingAccountIgnoresCancelWindowUpdatesAfterClose(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		emitPositionOnCancel: &exchanges.Position{
			Symbol:   "ETH",
			Quantity: decimal.RequireFromString("1"),
		},
	}
	acct := exchanges.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(context.Background()))

	positionSub := acct.SubscribePositions()
	defer positionSub.Unsubscribe()

	acct.Close()

	require.Eventually(t, func() bool {
		return adp.positionCancelEmits.Load() == 1 && adp.watchPositionsCanceled.Load() == 1
	}, time.Second, 10*time.Millisecond)

	require.Never(t, func() bool {
		select {
		case position, ok := <-positionSub.C:
			return ok && position != nil && position.Symbol == "ETH"
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond)
}

func TestTradingAccountIgnoresLateUpdatesAfterClose(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		keepCanceledCallbacks: true,
	}
	acct := exchanges.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(context.Background()))

	adp.EmitOrder(&exchanges.Order{OrderID: "open-order", Status: exchanges.OrderStatusNew})
	adp.EmitPosition(&exchanges.Position{Symbol: "ETH", Quantity: decimal.RequireFromString("1")})

	require.Eventually(t, func() bool {
		order, orderOK := acct.OpenOrder("open-order")
		pos, posOK := acct.Position("ETH")
		return orderOK && posOK &&
			order.Status == exchanges.OrderStatusNew &&
			pos.Quantity.Equal(decimal.RequireFromString("1"))
	}, time.Second, 10*time.Millisecond)

	acct.Close()

	adp.EmitOrder(&exchanges.Order{OrderID: "late-order", Status: exchanges.OrderStatusNew})
	adp.EmitPosition(&exchanges.Position{Symbol: "BTC", Quantity: decimal.RequireFromString("2")})

	require.Never(t, func() bool {
		_, orderOK := acct.OpenOrder("late-order")
		_, posOK := acct.Position("BTC")
		return orderOK || posOK
	}, 100*time.Millisecond, 10*time.Millisecond)
}

func TestTradingAccountIgnoresStaleCallbacksAfterRestart(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		keepCanceledCallbacks: true,
	}
	acct := exchanges.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(context.Background()))

	acct.Close()

	require.Eventually(t, func() bool {
		return adp.watchOrdersCanceled.Load() == 1 && adp.watchPositionsCanceled.Load() == 1
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	adp.EmitStaleOrder(&exchanges.Order{
		OrderID: "stale-order",
		Status:  exchanges.OrderStatusNew,
	})

	require.Never(t, func() bool {
		_, ok := acct.OpenOrder("stale-order")
		return ok
	}, 100*time.Millisecond, 10*time.Millisecond)
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
