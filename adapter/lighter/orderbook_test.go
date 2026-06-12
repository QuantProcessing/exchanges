package lighter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOrderBookInitialSnapshotMarksReady(t *testing.T) {
	ob := NewOrderBook("BTC")

	err := ob.ProcessUpdate([]byte(`{
		"type":"subscribed/order_book",
		"timestamp":1700000000000,
		"order_book":{
			"nonce":10,
			"bids":[{"price":"100","size":"2"}],
			"asks":[{"price":"101","size":"3"}]
		}
	}`))
	require.NoError(t, err)
	require.True(t, ob.WaitReady(context.Background(), 10*time.Millisecond))

	book := ob.ToAdapterOrderBook(5)
	require.Equal(t, "BTC", book.Symbol)
	require.Len(t, book.Bids, 1)
	require.Len(t, book.Asks, 1)
	require.Equal(t, "100", book.Bids[0].Price.String())
	require.Equal(t, "101", book.Asks[0].Price.String())
}

func TestOrderBookAppliesValidDelta(t *testing.T) {
	ob := NewOrderBook("BTC")

	require.NoError(t, ob.ProcessUpdate([]byte(`{
		"type":"subscribed/order_book",
		"order_book":{
			"nonce":10,
			"bids":[{"price":"100","size":"2"}],
			"asks":[{"price":"101","size":"3"}]
		}
	}`)))

	require.NoError(t, ob.ProcessUpdate([]byte(`{
		"type":"update/order_book",
		"order_book":{
			"begin_nonce":10,
			"nonce":11,
			"bids":[{"price":"100","size":"1"}],
			"asks":[{"price":"102","size":"4"}]
		}
	}`)))

	book := ob.ToAdapterOrderBook(5)
	require.Len(t, book.Bids, 1)
	require.Len(t, book.Asks, 2)
	require.Equal(t, "1", book.Bids[0].Quantity.String())
	require.Equal(t, "101", book.Asks[0].Price.String())
	require.Equal(t, "102", book.Asks[1].Price.String())
}

func TestOrderBookGapRequiresResyncAndPreservesLastGoodBook(t *testing.T) {
	ob := NewOrderBook("BTC")

	require.NoError(t, ob.ProcessUpdate([]byte(`{
		"type":"subscribed/order_book",
		"order_book":{
			"nonce":10,
			"bids":[{"price":"100","size":"2"}],
			"asks":[{"price":"101","size":"3"}]
		}
	}`)))

	err := ob.ProcessUpdate([]byte(`{
		"type":"update/order_book",
		"order_book":{
			"begin_nonce":12,
			"nonce":13,
			"bids":[{"price":"100","size":"0.5"}]
		}
	}`))
	require.ErrorIs(t, err, ErrOrderBookResyncRequired)

	book := ob.ToAdapterOrderBook(5)
	require.Len(t, book.Bids, 1)
	require.Len(t, book.Asks, 1)
	require.Equal(t, "2", book.Bids[0].Quantity.String())
	require.Equal(t, "101", book.Asks[0].Price.String())
}

func TestOrderBookIgnoresDeltasWhileWaitingForResyncSnapshot(t *testing.T) {
	ob := NewOrderBook("BTC")

	require.NoError(t, ob.ProcessUpdate([]byte(`{
		"type":"subscribed/order_book",
		"order_book":{
			"nonce":10,
			"bids":[{"price":"100","size":"2"}],
			"asks":[{"price":"101","size":"3"}]
		}
	}`)))

	require.ErrorIs(t, ob.ProcessUpdate([]byte(`{
		"type":"update/order_book",
		"order_book":{
			"begin_nonce":12,
			"nonce":13,
			"bids":[{"price":"100","size":"0.5"}]
		}
	}`)), ErrOrderBookResyncRequired)

	err := ob.ProcessUpdate([]byte(`{
		"type":"update/order_book",
		"order_book":{
			"begin_nonce":13,
			"nonce":14,
			"bids":[{"price":"99","size":"7"}]
		}
	}`))
	require.ErrorIs(t, err, ErrOrderBookResyncRequired)

	book := ob.ToAdapterOrderBook(5)
	require.Len(t, book.Bids, 1)
	require.Len(t, book.Asks, 1)
	require.Equal(t, "100", book.Bids[0].Price.String())
	require.Equal(t, "2", book.Bids[0].Quantity.String())
}

func TestOrderBookAcceptsFreshSnapshotAfterResync(t *testing.T) {
	ob := NewOrderBook("BTC")

	require.NoError(t, ob.ProcessUpdate([]byte(`{
		"type":"subscribed/order_book",
		"order_book":{
			"nonce":10,
			"bids":[{"price":"100","size":"2"}],
			"asks":[{"price":"101","size":"3"}]
		}
	}`)))

	require.ErrorIs(t, ob.ProcessUpdate([]byte(`{
		"type":"update/order_book",
		"order_book":{
			"begin_nonce":12,
			"nonce":13,
			"bids":[{"price":"100","size":"0.5"}]
		}
	}`)), ErrOrderBookResyncRequired)

	require.NoError(t, ob.ProcessUpdate([]byte(`{
		"type":"subscribed/order_book",
		"order_book":{
			"nonce":22,
			"bids":[{"price":"99","size":"7"}],
			"asks":[{"price":"100","size":"4"}]
		}
	}`)))

	book := ob.ToAdapterOrderBook(5)
	require.Len(t, book.Bids, 1)
	require.Len(t, book.Asks, 1)
	require.Equal(t, "99", book.Bids[0].Price.String())
	require.Equal(t, "7", book.Bids[0].Quantity.String())
	require.Equal(t, "100", book.Asks[0].Price.String())
}

func TestOrderBookUsesLastUpdatedAtAsBookTimestamp(t *testing.T) {
	ob := NewOrderBook("BTC")

	require.NoError(t, ob.ProcessUpdate([]byte(`{
		"type":"subscribed/order_book",
		"timestamp":1700000000123,
		"last_updated_at":1700000000456000,
		"order_book":{
			"nonce":10,
			"timestamp":1700000000000,
			"last_updated_at":1700000000455000,
			"bids":[{"price":"100","size":"2"}],
			"asks":[{"price":"101","size":"3"}]
		}
	}`)))

	book := ob.ToAdapterOrderBook(5)
	require.Equal(t, int64(1700000000456), ob.Timestamp())
	require.Equal(t, int64(1700000000456), book.Timestamp)
}
