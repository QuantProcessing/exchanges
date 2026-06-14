package risk

import (
	"testing"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestEngineRejectsOrdersThatViolateInstrumentPrecisionAndNotional(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{
		MaxOrderNotional: decimal.RequireFromString("1000"),
	})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		Quantity:      decimal.RequireFromString("0.125"),
		Price:         decimal.RequireFromString("100.01"),
	}
	require.NoError(t, engine.Check(order))

	order.Price = decimal.RequireFromString("100.001")
	require.ErrorIs(t, engine.Check(order), model.ErrInvalidOrder)

	order.Price = decimal.RequireFromString("100.01")
	order.Quantity = decimal.RequireFromString("0.0005")
	require.ErrorIs(t, engine.Check(order), model.ErrInvalidOrder)

	order.Quantity = decimal.RequireFromString("20")
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineRejectsReduceOnlyOrdersThatIncreaseExposure(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{})

	buy := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-2",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		Quantity:      decimal.RequireFromString("0.5"),
		ReduceOnly:    true,
	}
	require.ErrorIs(t, engine.Check(buy), ErrRiskRejected)

	sell := buy
	sell.ClientOrderID = "client-3"
	sell.Side = model.OrderSideSell
	require.NoError(t, engine.Check(sell))

	sell.ClientOrderID = "client-4"
	sell.Quantity = decimal.RequireFromString("2")
	require.ErrorIs(t, engine.Check(sell), ErrRiskRejected)
}

func TestEngineTradingStateHaltedAndReducing(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-trading-state",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("0.5"),
	}

	halted := NewEngine(c, Config{TradingState: TradingStateHalted})
	require.ErrorIs(t, halted.Check(order), ErrRiskRejected)

	reducing := NewEngine(c, Config{TradingState: TradingStateReducing})
	require.ErrorIs(t, reducing.Check(order), ErrRiskRejected)

	order.Side = model.OrderSideSell
	require.NoError(t, reducing.Check(order))

	order.Quantity = decimal.RequireFromString("2")
	require.ErrorIs(t, reducing.Check(order), ErrRiskRejected)
}

func TestEngineRejectsMarketOrdersExceedingNotionalUsingCachedMarketPrice(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: inst.ID,
		Last:         decimal.RequireFromString("101"),
	}}))
	engine := NewEngine(c, Config{
		MaxOrderNotional: decimal.RequireFromString("1000"),
	})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-market-risk",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("10"),
	}

	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineRejectsMarketOrdersExceedingNotionalUsingQuoteTick(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: inst.ID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("2"),
		AskSize:      decimal.RequireFromString("2"),
	}}))
	engine := NewEngine(c, Config{
		MaxOrderNotional: decimal.RequireFromString("1000"),
	})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-quote-risk",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("10"),
	}

	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineRejectsOrderNotionalWhenPriceUnavailable(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{MaxOrderNotional: decimal.RequireFromString("1000")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-missing-price",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("1"),
	}

	err := engine.Check(order)
	require.ErrorIs(t, err, ErrRiskRejected)
	require.ErrorContains(t, err, "cannot estimate order notional")
}

func TestEngineRejectsOrdersExceedingProjectedPositionNotional(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("9"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{MaxPositionNotional: decimal.RequireFromString("1000")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-position-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("2"),
		Price:         decimal.RequireFromString("100"),
	}
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)

	order.Side = model.OrderSideSell
	require.NoError(t, engine.Check(order))
}

func TestEngineRejectsOrdersExceedingProjectedPositionNotionalIncludingOpenOrders(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutOrder(model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   inst.ID,
		OrderID:        "open-buy",
		ClientOrderID:  "open-buy-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("9"),
		LeavesQuantity: decimal.RequireFromString("9"),
		Price:          decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{MaxPositionNotional: decimal.RequireFromString("1000")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-position-open-order-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("2"),
		Price:         decimal.RequireFromString("100"),
	}
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineRejectsOrdersExceedingProjectedAccountExposure(t *testing.T) {
	c := cache.New()
	btc := riskInstrument()
	eth := riskInstrument()
	eth.ID = model.MustInstrumentID("ETH-USDT-PERP.BINANCE")
	eth.RawSymbol = "ETHUSDT"
	eth.Base = "ETH"
	require.NoError(t, c.PutInstrument(btc))
	require.NoError(t, c.PutInstrument(eth))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: btc.ID,
		PositionID:   model.PositionID(btc.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("4"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: eth.ID,
		PositionID:   model.PositionID(eth.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("200"),
	}))
	engine := NewEngine(c, Config{MaxAccountExposure: decimal.RequireFromString("700")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  btc.ID,
		ClientOrderID: "client-account-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("2"),
		Price:         decimal.RequireFromString("100"),
	}
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)

	order.Quantity = decimal.RequireFromString("1")
	require.NoError(t, engine.Check(order))
}

func TestEngineUsesQuoteTickMarksForProjectedAccountExposure(t *testing.T) {
	c := cache.New()
	btc := riskInstrument()
	eth := riskInstrument()
	eth.ID = model.MustInstrumentID("ETH-USDT-PERP.BINANCE")
	eth.RawSymbol = "ETHUSDT"
	eth.Base = "ETH"
	require.NoError(t, c.PutInstrument(btc))
	require.NoError(t, c.PutInstrument(eth))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: eth.ID,
		PositionID:   model.PositionID(eth.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: eth.ID,
		BidPrice:     decimal.RequireFromString("300"),
		AskPrice:     decimal.RequireFromString("301"),
		BidSize:      decimal.RequireFromString("2"),
		AskSize:      decimal.RequireFromString("2"),
	}}))
	engine := NewEngine(c, Config{MaxAccountExposure: decimal.RequireFromString("350")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  btc.ID,
		ClientOrderID: "client-account-quote-mark",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("100"),
	}

	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineConvertsProjectedAccountExposureToAccountBaseCurrency(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	inst.ID = model.MustInstrumentID("BTC-EUR-PERP.BINANCE")
	inst.RawSymbol = "BTCEUR"
	inst.Quote = "EUR"
	inst.Settle = "EUR"
	xrate := model.Instrument{
		ID:        model.MustInstrumentID("EUR-USD-SPOT.BINANCE"),
		RawSymbol: "EURUSD",
		Type:      model.InstrumentTypeSpot,
		Base:      "EUR",
		Quote:     "USD",
		PriceTick: decimal.RequireFromString("0.0001"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutInstrument(xrate))
	c.PutAccount(model.AccountSnapshot{
		AccountID:    "acct",
		Venue:        "BINANCE",
		Type:         model.AccountTypeMargin,
		BaseCurrency: "USD",
	})
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("9"),
		EntryPrice:   decimal.RequireFromString("10"),
	}))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: xrate.ID,
		BidPrice:     decimal.RequireFromString("1.20"),
		AskPrice:     decimal.RequireFromString("1.20"),
		BidSize:      decimal.RequireFromString("1000"),
		AskSize:      decimal.RequireFromString("1000"),
	}}))
	engine := NewEngine(c, Config{MaxAccountExposure: decimal.RequireFromString("115")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-base-exposure-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("10"),
	}

	err := engine.Check(order)
	require.ErrorIs(t, err, ErrRiskRejected)
	require.ErrorContains(t, err, "max account exposure exceeded")
}

func TestEngineRejectsOrdersExceedingAvailableInitialMargin(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	inst.MarginInit = decimal.RequireFromString("0.10")
	inst.MarginMaint = decimal.RequireFromString("0.05")
	require.NoError(t, c.PutInstrument(inst))
	c.PutAccount(model.AccountSnapshot{
		AccountID: "acct",
		Venue:     "BINANCE",
		Type:      model.AccountTypeMargin,
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "90",
			Locked:   "10",
			Total:    "100",
		}},
		Margins: []model.MarginBalance{{
			Currency:    "USDT",
			Initial:     "20",
			Maintenance: "10",
		}},
	})
	engine := NewEngine(c, Config{})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-margin-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("1000"),
	}
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)

	order.Quantity = decimal.RequireFromString("0.5")
	require.NoError(t, engine.Check(order))
}

func TestEngineRejectsOrdersExceedingAvailableInitialMarginIncludingOpenOrders(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	inst.MarginInit = decimal.RequireFromString("0.10")
	require.NoError(t, c.PutInstrument(inst))
	c.PutAccount(model.AccountSnapshot{
		AccountID: "acct",
		Venue:     "BINANCE",
		Type:      model.AccountTypeMargin,
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "100",
			Total:    "100",
		}},
	})
	require.NoError(t, c.PutOrder(model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   inst.ID,
		OrderID:        "open-margin-buy",
		ClientOrderID:  "open-margin-buy-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("9"),
		LeavesQuantity: decimal.RequireFromString("9"),
		Price:          decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-margin-open-order-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("2"),
		Price:         decimal.RequireFromString("100"),
	}
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineRejectsInvalidTimeInForce(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-bad-tif",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForce("until-bored"),
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("100"),
	}

	require.ErrorIs(t, engine.Check(order), model.ErrInvalidOrder)
}

func riskInstrument() model.Instrument {
	return model.Instrument{
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
}
