package cache

import (
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCacheStoresInstrumentAccountAndOrders(t *testing.T) {
	c := New()
	inst := model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}
	require.NoError(t, c.PutInstrument(inst))
	_, ok := c.Instrument(inst.ID)
	require.True(t, ok)

	account := model.AccountSnapshot{AccountID: "acct", Venue: "BINANCE"}
	c.PutAccount(account)
	gotAccount, ok := c.Account("acct")
	require.True(t, ok)
	require.Equal(t, account, gotAccount)

	order := model.OrderStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "order-1",
		Status:       model.OrderStatusAccepted,
	}
	require.NoError(t, c.PutOrder(order))
	gotOrder, ok := c.Order("acct", "order-1")
	require.True(t, ok)
	require.Equal(t, order, gotOrder)
}

func TestCacheIndexesOrdersByVenueAndClientID(t *testing.T) {
	c := New()
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	order := model.OrderStatusReport{
		AccountID:       "acct",
		InstrumentID:    instID,
		OrderID:         "order-1",
		VenueOrderID:    "venue-1",
		ClientOrderID:   "client-1",
		Status:          model.OrderStatusAccepted,
		Quantity:        decimal.RequireFromString("1"),
		FilledQuantity:  decimal.RequireFromString("0.25"),
		LeavesQuantity:  decimal.RequireFromString("0.75"),
		LastUpdatedTime: testTime,
	}

	require.NoError(t, c.PutOrder(order))

	gotByClient, ok := c.OrderByClientID("acct", "client-1")
	require.True(t, ok)
	require.Equal(t, order, gotByClient)

	gotByVenue, ok := c.OrderByVenueID("acct", "venue-1")
	require.True(t, ok)
	require.Equal(t, order, gotByVenue)

	open := c.OpenOrders("acct")
	require.Len(t, open, 1)
	require.Equal(t, model.OrderID("order-1"), open[0].OrderID)

	order.Status = model.OrderStatusFilled
	order.FilledQuantity = decimal.RequireFromString("1")
	order.LeavesQuantity = decimal.Zero
	require.NoError(t, c.PutOrder(order))
	require.Empty(t, c.OpenOrders("acct"))
}

func TestCacheStoresFillsPositionsAndDeduplicatesTrades(t *testing.T) {
	c := New()
	fill := model.FillReport{
		AccountID:    "acct",
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderID:      "order-1",
		TradeID:      "trade-1",
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("0.5"),
		Timestamp:    testTime,
	}

	stored, err := c.PutFill(fill)
	require.NoError(t, err)
	require.True(t, stored)

	stored, err = c.PutFill(fill)
	require.NoError(t, err)
	require.False(t, stored)

	fills := c.FillsForOrder("acct", "order-1")
	require.Len(t, fills, 1)
	require.Equal(t, fill, fills[0])

	position := model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: fill.InstrumentID,
		PositionID:   "BTC-USDT-SPOT.BINANCE",
		Quantity:     decimal.RequireFromString("0.5"),
		EntryPrice:   decimal.RequireFromString("100"),
		Timestamp:    testTime,
	}
	require.NoError(t, c.PutPosition(position))

	gotPosition, ok := c.Position("acct", "BTC-USDT-SPOT.BINANCE")
	require.True(t, ok)
	require.Equal(t, position, gotPosition)

	gotByInstrument, ok := c.PositionByInstrument("acct", fill.InstrumentID)
	require.True(t, ok)
	require.Equal(t, position, gotByInstrument)

	positions := c.PositionsForInstrument(fill.InstrumentID)
	require.Len(t, positions, 1)
	require.Equal(t, position, positions[0])
}

func TestCacheStoresLatestMarketEventsByInstrument(t *testing.T) {
	c := New()
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	ticker := model.Ticker{
		InstrumentID: instID,
		Bid:          decimal.RequireFromString("100"),
		Ask:          decimal.RequireFromString("101"),
		Last:         decimal.RequireFromString("100.5"),
	}
	book := model.OrderBook{
		InstrumentID: instID,
		Bids: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("100"),
			Size:  decimal.RequireFromString("1"),
		}},
		Asks: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("101"),
			Size:  decimal.RequireFromString("1.5"),
		}},
	}
	trade := model.TradeTick{
		InstrumentID:  instID,
		Price:         decimal.RequireFromString("100.25"),
		Size:          decimal.RequireFromString("0.2"),
		AggressorSide: model.AggressorSideBuyer,
		TradeID:       "venue-trade-1",
		Timestamp:     testTime,
	}
	quote := model.QuoteTick{
		InstrumentID: instID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1.5"),
		AskSize:      decimal.RequireFromString("2.5"),
		Timestamp:    testTime,
	}
	barType := model.NewTimeBarType(instID, time.Minute)
	bar := model.Bar{
		BarType:   barType,
		Open:      decimal.RequireFromString("100"),
		High:      decimal.RequireFromString("102"),
		Low:       decimal.RequireFromString("99"),
		Close:     decimal.RequireFromString("101"),
		Volume:    decimal.RequireFromString("12.5"),
		Timestamp: testTime,
	}

	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Ticker: &ticker}))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{OrderBook: &book}))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Trade: &trade}))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Quote: &quote}))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Bar: &bar}))

	gotTicker, ok := c.Ticker(instID)
	require.True(t, ok)
	require.Equal(t, ticker, gotTicker)
	gotBook, ok := c.OrderBook(instID)
	require.True(t, ok)
	require.Equal(t, book, gotBook)
	gotTrade, ok := c.TradeTick(instID)
	require.True(t, ok)
	require.Equal(t, trade, gotTrade)
	gotQuote, ok := c.QuoteTick(instID)
	require.True(t, ok)
	require.Equal(t, quote, gotQuote)
	gotBar, ok := c.Bar(barType)
	require.True(t, ok)
	require.Equal(t, bar, gotBar)
	latestBar, ok := c.LatestBar(instID)
	require.True(t, ok)
	require.Equal(t, bar, latestBar)
}

var testTime = time.Unix(100, 0)
