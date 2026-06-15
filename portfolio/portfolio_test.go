package portfolio

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPortfolioAppliesFillsSideAwareAndComputesExposure(t *testing.T) {
	c := cache.New()
	inst := model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypePerp,
		Base:      "BTC",
		Quote:     "USDT",
		Settle:    "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)

	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "buy-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
	}))
	position, ok := c.PositionByInstrument("acct", inst.ID)
	require.True(t, ok)
	require.Equal(t, model.PositionSideLong, position.Side)
	require.True(t, decimal.RequireFromString("1").Equal(position.Quantity))
	require.True(t, decimal.RequireFromString("100").Equal(position.EntryPrice))
	require.True(t, decimal.RequireFromString("100").Equal(p.Exposure("acct", "USDT")))

	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "sell-1",
		TradeID:      "trade-2",
		Side:         model.OrderSideSell,
		Price:        decimal.RequireFromString("110"),
		Quantity:     decimal.RequireFromString("0.4"),
	}))
	position, ok = c.PositionByInstrument("acct", inst.ID)
	require.True(t, ok)
	require.Equal(t, model.PositionSideLong, position.Side)
	require.True(t, decimal.RequireFromString("0.6").Equal(position.Quantity))
	require.True(t, decimal.RequireFromString("66").Equal(p.Exposure("acct", "USDT")))
}

func TestPortfolioUsesFillPositionIDForLegFills(t *testing.T) {
	c := cache.New()
	inst := model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypePerp,
		Base:      "BTC",
		Quote:     "USDT",
		Settle:    "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)

	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		VenueOrderID:  "venue-LEG-1",
		ClientOrderID: "spread-LEG-1",
		TradeID:       "trade-leg-1",
		PositionID:    "leg-position-1",
		IsLeg:         true,
		Side:          model.OrderSideBuy,
		Price:         decimal.RequireFromString("100"),
		Quantity:      decimal.RequireFromString("0.25"),
	}))

	position, ok := c.Position("acct", "leg-position-1")
	require.True(t, ok)
	require.Equal(t, inst.ID, position.InstrumentID)
	require.Equal(t, model.PositionSideLong, position.Side)
	require.True(t, decimal.RequireFromString("0.25").Equal(position.Quantity))
}

func TestPortfolioTracksRealizedUnrealizedPnLAndCommissions(t *testing.T) {
	c := cache.New()
	inst := model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypePerp,
		Base:      "BTC",
		Quote:     "USDT",
		Settle:    "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)

	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "buy-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
		Fee:          decimal.RequireFromString("0.10"),
		FeeCurrency:  "USDT",
	}))
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "sell-1",
		TradeID:      "trade-2",
		Side:         model.OrderSideSell,
		Price:        decimal.RequireFromString("110"),
		Quantity:     decimal.RequireFromString("0.4"),
		Fee:          decimal.RequireFromString("0.04"),
		FeeCurrency:  "USDT",
	}))
	p.SetMark("acct", inst.ID, decimal.RequireFromString("120"))

	require.Equal(t, "4", p.RealizedPnL("acct", inst.ID).String())
	require.Equal(t, "12", p.UnrealizedPnL("acct", inst.ID).String())
	require.Equal(t, "0.14", p.Commission("acct", "USDT").String())
}

func TestPortfolioRecordsClosedTradeWithAccountCurrencyPnL(t *testing.T) {
	c := cache.New()
	inst := portfolioInstrument(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), "BTC")
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)
	analyzer := &recordingAnalyzer{}
	p.SetAnalyzer(analyzer)
	require.NoError(t, p.SetConversionRate("USDT", "USD", decimal.RequireFromString("1.10")))
	require.NoError(t, p.UpdateAccount(model.AccountSnapshot{
		AccountID:    "acct",
		Venue:        "BINANCE",
		Type:         model.AccountTypeMargin,
		BaseCurrency: "USD",
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "1000",
			Total:    "1000",
		}},
	}))
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "buy-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
	}))
	require.Empty(t, analyzer.records)

	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "sell-1",
		TradeID:      "trade-2",
		Side:         model.OrderSideSell,
		Price:        decimal.RequireFromString("110"),
		Quantity:     decimal.RequireFromString("1"),
	}))
	require.Len(t, analyzer.records, 1)
	record := analyzer.records[0]
	require.Equal(t, model.AccountID("acct"), record.AccountID)
	require.Equal(t, inst.ID, record.InstrumentID)
	require.Equal(t, model.PositionID(inst.ID.String()), record.PositionID)
	require.Equal(t, model.Currency("USDT"), record.Currency)
	require.True(t, decimal.RequireFromString("10").Equal(record.RealizedPnL))
	require.Equal(t, model.Currency("USD"), record.AccountCurrency)
	require.True(t, decimal.RequireFromString("11").Equal(record.AccountCurrencyPnL))
}

func TestPortfolioAppliesFillBalanceDeltasForCommissionAndRealizedPnL(t *testing.T) {
	c := cache.New()
	inst := portfolioInstrument(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), "BTC")
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)
	require.NoError(t, p.UpdateAccount(model.AccountSnapshot{
		AccountID: "acct",
		Venue:     "BINANCE",
		Type:      model.AccountTypeMargin,
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "1000",
			Total:    "1000",
		}},
	}))

	open := model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "buy-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
		Fee:          decimal.RequireFromString("0.10"),
		FeeCurrency:  "USDT",
	}
	require.NoError(t, p.ApplyFill(open))
	account, ok := c.Account("acct")
	require.True(t, ok)
	require.Equal(t, "999.9", account.Balances[0].Total)
	require.Equal(t, "999.9", account.Balances[0].Free)

	close := model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "sell-1",
		TradeID:      "trade-2",
		Side:         model.OrderSideSell,
		Price:        decimal.RequireFromString("110"),
		Quantity:     decimal.RequireFromString("0.4"),
		Fee:          decimal.RequireFromString("0.04"),
		FeeCurrency:  "USDT",
	}
	require.NoError(t, p.ApplyFill(close))
	account, ok = c.Account("acct")
	require.True(t, ok)
	require.Equal(t, "1003.86", account.Balances[0].Total)
	require.Equal(t, "1003.86", account.Balances[0].Free)

	require.NoError(t, p.ApplyFill(close))
	account, ok = c.Account("acct")
	require.True(t, ok)
	require.Equal(t, "1003.86", account.Balances[0].Total)
	require.Equal(t, "1003.86", account.Balances[0].Free)
}

func TestPortfolioTracksAccountBalancesMarginsAndEquity(t *testing.T) {
	c := cache.New()
	inst := portfolioInstrument(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), "BTC")
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)
	snapshot := model.AccountSnapshot{
		AccountID: "acct",
		Venue:     "BINANCE",
		Type:      model.AccountTypeMargin,
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "900",
			Locked:   "100",
			Total:    "1000",
		}},
		Margins: []model.MarginBalance{{
			Currency:     "USDT",
			InstrumentID: inst.ID,
			Initial:      "125",
			Maintenance:  "75",
		}},
	}

	require.NoError(t, p.UpdateAccount(snapshot))
	require.Equal(t, "100", p.BalancesLocked("acct")["USDT"].String())
	require.Equal(t, "125", p.MarginsInit("acct")[inst.ID].String())
	require.Equal(t, "75", p.MarginsMaint("acct")[inst.ID].String())

	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "buy-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
	}))
	p.SetMark("acct", inst.ID, decimal.RequireFromString("120"))

	require.Equal(t, "1020", p.Equity("acct")["USDT"].String())
	require.Equal(t, "795", p.AvailableEquity("acct")["USDT"].String())
}

func TestPortfolioHandlesExecutionEventsForAccountOrderFillAndPosition(t *testing.T) {
	c := cache.New()
	inst := portfolioInstrument(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), "BTC")
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)

	account := model.AccountSnapshot{
		AccountID: "acct",
		Venue:     "BINANCE",
		Type:      model.AccountTypeMargin,
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "1000",
			Total:    "1000",
		}},
	}
	require.NoError(t, p.HandleExecutionEvent(model.ExecutionEvent{Account: &account}))
	_, ok := c.Account("acct")
	require.True(t, ok)

	order := model.OrderStatusReport{
		Metadata:       model.CommandMetadata{StrategyID: "strategy-001"},
		AccountID:      "acct",
		InstrumentID:   inst.ID,
		OrderID:        "order-1",
		ClientOrderID:  "client-1",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
		Price:          decimal.RequireFromString("100"),
	}
	require.NoError(t, p.HandleExecutionEvent(model.ExecutionEvent{Order: &order}))
	cachedOrder, ok := c.OrderByClientID("acct", "client-1")
	require.True(t, ok)
	require.Equal(t, model.StrategyID("strategy-001"), cachedOrder.Metadata.StrategyID)

	fill := model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "order-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
		Fee:          decimal.RequireFromString("0.10"),
		FeeCurrency:  "USDT",
	}
	require.NoError(t, p.HandleExecutionEvent(model.ExecutionEvent{Fill: &fill}))
	position, ok := c.PositionByInstrument("acct", inst.ID)
	require.True(t, ok)
	require.Equal(t, model.PositionSideLong, position.Side)
	require.True(t, decimal.RequireFromString("0.10").Equal(p.Commission("acct", "USDT")))

	reportedPosition := model.PositionStatusReport{
		Metadata:        model.CommandMetadata{StrategyID: "strategy-001"},
		AccountID:       "acct",
		InstrumentID:    inst.ID,
		PositionID:      "venue-position-1",
		VenuePositionID: "venue-position-1",
		Side:            model.PositionSideLong,
		Quantity:        decimal.RequireFromString("0.5"),
		EntryPrice:      decimal.RequireFromString("101"),
	}
	require.NoError(t, p.HandleExecutionEvent(model.ExecutionEvent{Position: &reportedPosition}))
	position, ok = c.PositionByInstrument("acct", inst.ID)
	require.True(t, ok)
	require.Equal(t, model.PositionID("venue-position-1"), position.PositionID)
	require.Equal(t, "0.5", position.Quantity.String())
}

func TestPortfolioConvertsEquityToAccountBaseCurrency(t *testing.T) {
	c := cache.New()
	xrate := portfolioSpotInstrument(model.MustInstrumentID("EUR-USD-SPOT.BINANCE"), "EUR", "USD")
	require.NoError(t, c.PutInstrument(xrate))
	p := New(c)

	require.NoError(t, p.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: xrate.ID,
		BidPrice:     decimal.RequireFromString("1.10"),
		AskPrice:     decimal.RequireFromString("1.10"),
		BidSize:      decimal.RequireFromString("1000"),
		AskSize:      decimal.RequireFromString("1000"),
	}}))
	require.NoError(t, p.UpdateAccount(model.AccountSnapshot{
		AccountID:    "acct",
		Venue:        "BINANCE",
		Type:         model.AccountTypeMargin,
		BaseCurrency: "USD",
		Balances:     []model.Balance{{Currency: "EUR", Free: "100", Total: "100"}},
		Margins:      []model.MarginBalance{{Currency: "EUR", Initial: "10"}},
		Timestamp:    timeNowForPortfolioTests(),
	}))

	equity := p.Equity("acct")
	require.Len(t, equity, 1)
	require.Equal(t, "110", equity["USD"].String())
	available := p.AvailableEquity("acct")
	require.Len(t, available, 1)
	require.Equal(t, "99", available["USD"].String())
}

func TestPortfolioConvertsExposureToTargetCurrency(t *testing.T) {
	c := cache.New()
	inst := model.Instrument{
		ID:        model.MustInstrumentID("BTC-EUR-PERP.BINANCE"),
		RawSymbol: "BTCEUR",
		Type:      model.InstrumentTypePerp,
		Base:      "BTC",
		Quote:     "EUR",
		Settle:    "EUR",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}
	xrate := portfolioSpotInstrument(model.MustInstrumentID("EUR-USD-SPOT.BINANCE"), "EUR", "USD")
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutInstrument(xrate))
	p := New(c)
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "buy-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
	}))
	require.NoError(t, p.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: xrate.ID,
		BidPrice:     decimal.RequireFromString("1.10"),
		AskPrice:     decimal.RequireFromString("1.10"),
		BidSize:      decimal.RequireFromString("1000"),
		AskSize:      decimal.RequireFromString("1000"),
	}}))

	require.Equal(t, "110", p.Exposure("acct", "USD").String())
}

func TestPortfolioUsesExplicitConversionRatesForSettleAndAccountBase(t *testing.T) {
	c := cache.New()
	inst := model.Instrument{
		ID:        model.MustInstrumentID("BTC-EUR-PERP.BINANCE"),
		RawSymbol: "BTCEUR",
		Type:      model.InstrumentTypePerp,
		Base:      "BTC",
		Quote:     "EUR",
		Settle:    "EUR",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)
	require.NoError(t, p.SetConversionRate("EUR", "USD", decimal.RequireFromString("1.20")))
	require.NoError(t, p.UpdateAccount(model.AccountSnapshot{
		AccountID:    "acct",
		Venue:        "BINANCE",
		Type:         model.AccountTypeMargin,
		BaseCurrency: "USD",
		Balances: []model.Balance{{
			Currency: "EUR",
			Free:     "100",
			Total:    "100",
		}},
	}))
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "buy-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
	}))
	p.SetMark("acct", inst.ID, decimal.RequireFromString("120"))

	require.Equal(t, "144", p.Exposure("acct", "USD").String())
	require.Equal(t, "144", p.Equity("acct")["USD"].String())
}

func TestPortfolioInvalidatesUnrealizedPnLCacheOnOrderPositionAccountAndMarketEvents(t *testing.T) {
	c := cache.New()
	inst := portfolioInstrument(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), "BTC")
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "buy-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
	}))
	p.SetMark("acct", inst.ID, decimal.RequireFromString("110"))
	require.Equal(t, "10", p.UnrealizedPnL("acct", inst.ID).String())
	requireUnrealizedCached(t, p, "acct", inst.ID)

	order := model.OrderStatusReport{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		OrderID:       "order-1",
		ClientOrderID: "client-1",
		Status:        model.OrderStatusAccepted,
	}
	require.NoError(t, p.HandleExecutionEvent(model.ExecutionEvent{Order: &order}))
	requireUnrealizedNotCached(t, p, "acct", inst.ID)
	require.Equal(t, "10", p.UnrealizedPnL("acct", inst.ID).String())
	requireUnrealizedCached(t, p, "acct", inst.ID)

	account := model.AccountSnapshot{
		AccountID: "acct",
		Venue:     "BINANCE",
		Type:      model.AccountTypeMargin,
		Balances:  []model.Balance{{Currency: "USDT", Free: "1000", Total: "1000"}},
	}
	require.NoError(t, p.HandleExecutionEvent(model.ExecutionEvent{Account: &account}))
	requireUnrealizedNotCached(t, p, "acct", inst.ID)
	require.Equal(t, "10", p.UnrealizedPnL("acct", inst.ID).String())
	requireUnrealizedCached(t, p, "acct", inst.ID)

	require.NoError(t, p.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: inst.ID,
		BidPrice:     decimal.RequireFromString("120"),
		AskPrice:     decimal.RequireFromString("121"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}))
	requireUnrealizedNotCached(t, p, "acct", inst.ID)
	require.Equal(t, "20", p.UnrealizedPnL("acct", inst.ID).String())
	requireUnrealizedCached(t, p, "acct", inst.ID)

	position := model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   "venue-position-1",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("2"),
		EntryPrice:   decimal.RequireFromString("100"),
	}
	require.NoError(t, p.HandleExecutionEvent(model.ExecutionEvent{Position: &position}))
	requireUnrealizedNotCached(t, p, "acct", inst.ID)
	require.Equal(t, "40", p.UnrealizedPnL("acct", inst.ID).String())
}

func TestPortfolioAggregatesNetPositionsAndExposureByInstrumentAccountVenueAndCurrency(t *testing.T) {
	c := cache.New()
	btc := portfolioInstrument(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), "BTC")
	eth := portfolioInstrument(model.MustInstrumentID("ETH-USDT-PERP.OKX"), "ETH")
	xrate := portfolioSpotInstrument(model.MustInstrumentID("USDT-USD-SPOT.BINANCE"), "USDT", "USD")
	require.NoError(t, c.PutInstrument(btc))
	require.NoError(t, c.PutInstrument(eth))
	require.NoError(t, c.PutInstrument(xrate))
	p := New(c)
	require.NoError(t, p.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: xrate.ID,
		BidPrice:     decimal.RequireFromString("1.10"),
		AskPrice:     decimal.RequireFromString("1.10"),
		BidSize:      decimal.RequireFromString("1000"),
		AskSize:      decimal.RequireFromString("1000"),
	}}))
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct-a",
		InstrumentID: btc.ID,
		OrderID:      "acct-a-btc",
		TradeID:      "acct-a-btc-trade",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
	}))
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct-b",
		InstrumentID: btc.ID,
		OrderID:      "acct-b-btc",
		TradeID:      "acct-b-btc-trade",
		Side:         model.OrderSideSell,
		Price:        decimal.RequireFromString("120"),
		Quantity:     decimal.RequireFromString("0.4"),
	}))
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct-a",
		InstrumentID: eth.ID,
		OrderID:      "acct-a-eth",
		TradeID:      "acct-a-eth-trade",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("50"),
		Quantity:     decimal.RequireFromString("2"),
	}))
	p.SetMark("acct-a", btc.ID, decimal.RequireFromString("110"))
	p.SetMark("acct-b", btc.ID, decimal.RequireFromString("110"))
	p.SetMark("acct-a", eth.ID, decimal.RequireFromString("55"))

	require.True(t, decimal.RequireFromString("1").Equal(p.NetPosition("acct-a", btc.ID)))
	require.True(t, decimal.RequireFromString("0.6").Equal(p.NetPosition("", btc.ID)))
	require.True(t, decimal.RequireFromString("0.6").Equal(p.NetPositionsByInstrument("")[btc.ID]))
	require.True(t, decimal.RequireFromString("2").Equal(p.NetPositionsByInstrument("")[eth.ID]))
	require.True(t, decimal.RequireFromString("72.6").Equal(p.NetExposuresByInstrument("", "USD")[btc.ID]))
	require.True(t, decimal.RequireFromString("121").Equal(p.NetExposuresByInstrument("", "USD")[eth.ID]))
	require.True(t, decimal.RequireFromString("242").Equal(p.NetExposuresByAccount("USD")["acct-a"]))
	require.True(t, decimal.RequireFromString("-48.4").Equal(p.NetExposuresByAccount("USD")["acct-b"]))
	require.True(t, decimal.RequireFromString("72.6").Equal(p.NetExposuresByVenue("USD")["BINANCE"]))
	require.True(t, decimal.RequireFromString("121").Equal(p.NetExposuresByVenue("USD")["OKX"]))
}

func TestPortfolioAppliesQuoteTickMarksForUnrealizedPnL(t *testing.T) {
	c := cache.New()
	inst := model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypePerp,
		Base:      "BTC",
		Quote:     "USDT",
		Settle:    "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "buy-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
	}))

	require.NoError(t, p.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: inst.ID,
		BidPrice:     decimal.RequireFromString("120"),
		AskPrice:     decimal.RequireFromString("121"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}))

	require.Equal(t, "20", p.UnrealizedPnL("acct", inst.ID).String())
	require.Equal(t, "120", p.Exposure("acct", "USDT").String())
}

func TestPortfolioConcurrentFillAndMarketUpdates(t *testing.T) {
	c := cache.New()
	inst := portfolioInstrument(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), "BTC")
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			require.NoError(t, p.ApplyFill(model.FillReport{
				AccountID:    "acct",
				InstrumentID: inst.ID,
				OrderID:      model.OrderID(fmt.Sprintf("order-%d", i)),
				TradeID:      model.TradeID(fmt.Sprintf("trade-%d", i)),
				Side:         model.OrderSideBuy,
				Price:        decimal.RequireFromString("100"),
				Quantity:     decimal.RequireFromString("0.01"),
			}))
		}(i)
		go func() {
			defer wg.Done()
			require.NoError(t, p.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
				InstrumentID: inst.ID,
				BidPrice:     decimal.RequireFromString("101"),
				AskPrice:     decimal.RequireFromString("102"),
				BidSize:      decimal.RequireFromString("1"),
				AskSize:      decimal.RequireFromString("1"),
			}}))
		}()
	}
	wg.Wait()

	position, ok := c.PositionByInstrument("acct", inst.ID)
	require.True(t, ok)
	require.True(t, position.Quantity.IsPositive())
	require.True(t, p.Exposure("acct", "USDT").IsPositive())
}

func TestPortfolioMarkValuesUseSignedLongShortContributions(t *testing.T) {
	c := cache.New()
	btc := portfolioInstrument(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), "BTC")
	eth := portfolioInstrument(model.MustInstrumentID("ETH-USDT-PERP.BINANCE"), "ETH")
	require.NoError(t, c.PutInstrument(btc))
	require.NoError(t, c.PutInstrument(eth))
	p := New(c)

	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: btc.ID,
		OrderID:      "btc-buy",
		TradeID:      "btc-trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
	}))
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: eth.ID,
		OrderID:      "eth-sell",
		TradeID:      "eth-trade-1",
		Side:         model.OrderSideSell,
		Price:        decimal.RequireFromString("50"),
		Quantity:     decimal.RequireFromString("2"),
	}))
	require.NoError(t, p.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: btc.ID,
		BidPrice:     decimal.RequireFromString("120"),
		AskPrice:     decimal.RequireFromString("121"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}))
	require.NoError(t, p.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: eth.ID,
		BidPrice:     decimal.RequireFromString("45"),
		AskPrice:     decimal.RequireFromString("46"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}))

	require.Equal(t, "120", p.MarkValue("acct", btc.ID).String())
	require.Equal(t, "-92", p.MarkValue("acct", eth.ID).String())
	require.Equal(t, "28", p.MarkValues("acct")["USDT"].String())
	require.Equal(t, "212", p.Exposure("acct", "USDT").String())
}

func TestPortfolioRealizedPnLWhenFillFlipsPosition(t *testing.T) {
	c := cache.New()
	inst := model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypePerp,
		Base:      "BTC",
		Quote:     "USDT",
		Settle:    "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}
	require.NoError(t, c.PutInstrument(inst))
	p := New(c)

	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "buy-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
	}))
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		OrderID:      "sell-1",
		TradeID:      "trade-2",
		Side:         model.OrderSideSell,
		Price:        decimal.RequireFromString("90"),
		Quantity:     decimal.RequireFromString("1.5"),
	}))

	position, ok := c.PositionByInstrument("acct", inst.ID)
	require.True(t, ok)
	require.Equal(t, model.PositionSideShort, position.Side)
	require.Equal(t, "0.5", position.Quantity.String())
	require.Equal(t, "90", position.EntryPrice.String())
	require.Equal(t, "-10", p.RealizedPnL("acct", inst.ID).String())
}

func portfolioInstrument(instrumentID model.InstrumentID, base model.Currency) model.Instrument {
	return model.Instrument{
		ID:        instrumentID,
		RawSymbol: string(base) + "USDT",
		Type:      model.InstrumentTypePerp,
		Base:      base,
		Quote:     "USDT",
		Settle:    "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}
}

func portfolioSpotInstrument(instrumentID model.InstrumentID, base model.Currency, quote model.Currency) model.Instrument {
	return model.Instrument{
		ID:        instrumentID,
		RawSymbol: string(base) + string(quote),
		Type:      model.InstrumentTypeSpot,
		Base:      base,
		Quote:     quote,
		PriceTick: decimal.RequireFromString("0.0001"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}
}

func requireUnrealizedCached(t *testing.T, p *Portfolio, accountID model.AccountID, instrumentID model.InstrumentID) {
	t.Helper()
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.unrealizedCache[accountID] == nil {
		t.Fatalf("unrealized cache missing for account %s", accountID)
	}
	if _, ok := p.unrealizedCache[accountID][instrumentID]; !ok {
		t.Fatalf("unrealized cache missing for instrument %s", instrumentID.String())
	}
}

func requireUnrealizedNotCached(t *testing.T, p *Portfolio, accountID model.AccountID, instrumentID model.InstrumentID) {
	t.Helper()
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.unrealizedCache[accountID] == nil {
		return
	}
	if _, ok := p.unrealizedCache[accountID][instrumentID]; ok {
		t.Fatalf("unrealized cache still present for instrument %s", instrumentID.String())
	}
}

type recordingAnalyzer struct {
	records []TradeRecord
}

func (a *recordingAnalyzer) RecordTrade(record TradeRecord) {
	a.records = append(a.records, record)
}

func timeNowForPortfolioTests() time.Time {
	return time.Unix(100, 0)
}
