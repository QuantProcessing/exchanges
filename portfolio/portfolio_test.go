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

func timeNowForPortfolioTests() time.Time {
	return time.Unix(100, 0)
}
