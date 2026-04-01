package backpack

import (
	"context"
	"strconv"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestEmitOrderBookUpdateDoesNotTruncateDepth(t *testing.T) {
	t.Parallel()

	ob := &OrderBook{
		symbol:      "BTC_USDC",
		bids:        make(map[string]decimal.Decimal, 25),
		asks:        make(map[string]decimal.Decimal, 25),
		initialized: true,
	}

	for i := 0; i < 25; i++ {
		bidPrice := decimal.NewFromInt(50000 - int64(i))
		askPrice := decimal.NewFromInt(50001 + int64(i))
		qty := decimal.NewFromInt(int64(i + 1))
		ob.bids[bidPrice.String()] = qty
		ob.asks[askPrice.String()] = qty
	}

	var update *exchanges.OrderBook
	emitOrderBookUpdate(func(book *exchanges.OrderBook) {
		update = book
	}, ob, "btc_usdc", 1710000000000000, 0)

	require.NotNil(t, update)
	require.Equal(t, "BTC_USDC", update.Symbol)
	require.Len(t, update.Bids, 25)
	require.Len(t, update.Asks, 25)
	require.Equal(t, "50000", update.Bids[0].Price.String())
	require.Equal(t, "50025", update.Asks[24].Price.String())
	require.Equal(t, strconv.FormatInt(1710000000000000/1000, 10), strconv.FormatInt(update.Timestamp, 10))
}

func TestEmitOrderBookUpdateUsesRequestedDepth(t *testing.T) {
	t.Parallel()

	ob := &OrderBook{
		symbol:      "BTC_USDC",
		bids:        make(map[string]decimal.Decimal, 25),
		asks:        make(map[string]decimal.Decimal, 25),
		initialized: true,
	}

	for i := 0; i < 25; i++ {
		bidPrice := decimal.NewFromInt(50000 - int64(i))
		askPrice := decimal.NewFromInt(50001 + int64(i))
		qty := decimal.NewFromInt(int64(i + 1))
		ob.bids[bidPrice.String()] = qty
		ob.asks[askPrice.String()] = qty
	}

	var update *exchanges.OrderBook
	emitOrderBookUpdate(func(book *exchanges.OrderBook) {
		update = book
	}, ob, "btc_usdc", 1710000000000000, 10)

	require.NotNil(t, update)
	require.Len(t, update.Bids, 10)
	require.Len(t, update.Asks, 10)
	require.Equal(t, "50000", update.Bids[0].Price.String())
	require.Equal(t, "50010", update.Asks[9].Price.String())
}

type sequenceDepthClient struct {
	snapshots []*sdk.Depth
	index     int
}

func (c *sequenceDepthClient) GetOrderBook(ctx context.Context, symbol string, limit int) (*sdk.Depth, error) {
	_ = ctx
	_ = symbol
	_ = limit

	snapshot := c.snapshots[c.index]
	if c.index < len(c.snapshots)-1 {
		c.index++
	}
	return snapshot, nil
}

func TestWaitForInitialOrderBookSnapshotRetriesTooOldSnapshots(t *testing.T) {
	t.Parallel()

	ob := NewOrderBook("BTC_USDC")
	require.NoError(t, ob.ProcessUpdate(&sdk.DepthEvent{
		FirstUpdateID: 100,
		FinalUpdateID: 100,
		Bids:          [][]string{{"49995", "1"}},
		Asks:          [][]string{{"50015", "4"}},
	}))

	client := &sequenceDepthClient{
		snapshots: []*sdk.Depth{
			{
				LastUpdateID: "100",
				Bids:         [][]string{{"49990", "1"}},
				Asks:         [][]string{{"50020", "4"}},
				Timestamp:    1710000000000,
			},
		},
	}

	go func() {
		<-time.After(50 * time.Millisecond)
		_ = ob.ProcessUpdate(&sdk.DepthEvent{
			FirstUpdateID: 101,
			FinalUpdateID: 102,
			Bids:          [][]string{{"50000", "2"}},
			Asks:          [][]string{{"50010", "3"}},
		})
	}()

	err := waitForInitialOrderBookSnapshot(context.Background(), client, "BTC_USDC", ob)
	require.NoError(t, err)
	require.True(t, ob.IsInitialized())

	bids, asks := ob.GetDepth(1)
	require.Len(t, bids, 1)
	require.Len(t, asks, 1)
	require.Equal(t, "50000", bids[0].Price.String())
	require.Equal(t, "2", bids[0].Quantity.String())
}
