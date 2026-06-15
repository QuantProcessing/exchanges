package model

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestInstrumentIDParseAndValidate(t *testing.T) {
	id, err := ParseInstrumentID("BTC-USDT-SPOT.BINANCE")
	require.NoError(t, err)
	require.Equal(t, InstrumentID{Symbol: "BTC-USDT-SPOT", Venue: Venue("BINANCE")}, id)
	require.Equal(t, "BTC-USDT-SPOT.BINANCE", id.String())

	_, err = ParseInstrumentID("BTCUSDT")
	require.ErrorIs(t, err, ErrInvalidInstrumentID)
}

func TestInstrumentValidateRequiresTradingShape(t *testing.T) {
	inst := Instrument{
		ID:        MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      InstrumentTypePerp,
		Base:      Currency("BTC"),
		Quote:     Currency("USDT"),
		Settle:    Currency("USDT"),
		PriceTick: decimal.RequireFromString("0.1"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    InstrumentStatusTrading,
	}
	require.NoError(t, inst.Validate())

	inst.RawSymbol = ""
	require.ErrorIs(t, inst.Validate(), ErrInvalidInstrument)
}

func TestInstrumentValidateAllowsNonNegativeFeesAndMarginRates(t *testing.T) {
	inst := Instrument{
		ID:          MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol:   "BTCUSDT",
		Type:        InstrumentTypePerp,
		Base:        Currency("BTC"),
		Quote:       Currency("USDT"),
		Settle:      Currency("USDT"),
		PriceTick:   decimal.RequireFromString("0.1"),
		SizeTick:    decimal.RequireFromString("0.001"),
		MakerFee:    decimal.RequireFromString("0.0002"),
		TakerFee:    decimal.RequireFromString("0.0005"),
		MarginInit:  decimal.RequireFromString("0.1"),
		MarginMaint: decimal.RequireFromString("0.05"),
		Status:      InstrumentStatusTrading,
	}
	require.NoError(t, inst.Validate())

	inst.MarginInit = decimal.RequireFromString("-0.1")
	require.ErrorIs(t, inst.Validate(), ErrInvalidInstrument)

	inst.MarginInit = decimal.RequireFromString("0.1")
	inst.TakerFee = decimal.RequireFromString("-0.0005")
	require.ErrorIs(t, inst.Validate(), ErrInvalidInstrument)
}

func TestInstrumentPrecisionValidation(t *testing.T) {
	inst := Instrument{
		ID:        MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      InstrumentTypeSpot,
		Base:      Currency("BTC"),
		Quote:     Currency("USDT"),
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    InstrumentStatusTrading,
	}
	require.NoError(t, inst.ValidatePrice(decimal.RequireFromString("100.01")))
	require.ErrorIs(t, inst.ValidatePrice(decimal.RequireFromString("100.001")), ErrInvalidOrder)
	require.NoError(t, inst.ValidateSize(decimal.RequireFromString("0.125")))
	require.ErrorIs(t, inst.ValidateSize(decimal.RequireFromString("0.0005")), ErrInvalidOrder)
}

func TestSubmitOrderSupportsProductionOrderSemantics(t *testing.T) {
	stopLimit := SubmitOrder{
		AccountID:     AccountID("acct"),
		InstrumentID:  MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		ClientOrderID: ClientOrderID("client-stop-1"),
		Side:          OrderSideBuy,
		Type:          OrderTypeStopLimit,
		TimeInForce:   TimeInForceGTD,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
		TriggerPrice:  decimal.RequireFromString("99"),
		PostOnly:      true,
		ReduceOnly:    true,
		ExpireTime:    testNow.Add(time.Hour),
	}
	require.NoError(t, stopLimit.Validate())

	stopLimit.TriggerPrice = decimal.Zero
	require.ErrorIs(t, stopLimit.Validate(), ErrInvalidOrder)

	trailing := stopLimit
	trailing.Type = OrderTypeTrailingStopMarket
	trailing.Price = decimal.Zero
	trailing.TriggerPrice = decimal.Zero
	trailing.TimeInForce = TimeInForceGTD
	trailing.PostOnly = false
	trailing.TrailingOffset = decimal.RequireFromString("10")
	trailing.ActivationPrice = decimal.RequireFromString("95")
	require.NoError(t, trailing.Validate())
}

func TestSubmitOrderRequiresExpireTimeOnlyForGTD(t *testing.T) {
	order := SubmitOrder{
		AccountID:     AccountID("acct"),
		InstrumentID:  MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		ClientOrderID: ClientOrderID("client-gtd-1"),
		Side:          OrderSideBuy,
		Type:          OrderTypeLimit,
		TimeInForce:   TimeInForceGTD,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
	}
	require.ErrorIs(t, order.Validate(), ErrInvalidOrder)

	order.ExpireTime = time.Unix(0, 0)
	require.ErrorIs(t, order.Validate(), ErrInvalidOrder)

	order.ExpireTime = time.Unix(1, 0)
	require.NoError(t, order.Validate())

	order.TimeInForce = TimeInForceGTC
	require.ErrorIs(t, order.Validate(), ErrInvalidOrder)
}

func TestSubmitOrderRejectsGTDMarketOrders(t *testing.T) {
	order := SubmitOrder{
		AccountID:     AccountID("acct"),
		InstrumentID:  MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		ClientOrderID: ClientOrderID("client-market-gtd"),
		Side:          OrderSideBuy,
		Type:          OrderTypeMarket,
		TimeInForce:   TimeInForceGTD,
		Quantity:      decimal.RequireFromString("0.5"),
		ExpireTime:    time.Unix(1, 0),
	}
	require.ErrorIs(t, order.Validate(), ErrInvalidOrder)
}

func TestSubmitOrderRejectsInvalidOrderType(t *testing.T) {
	order := SubmitOrder{
		AccountID:     AccountID("acct"),
		InstrumentID:  MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		ClientOrderID: ClientOrderID("client-invalid-type"),
		Side:          OrderSideBuy,
		Type:          OrderType("because-i-said-so"),
		TimeInForce:   TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("100"),
	}
	require.ErrorIs(t, order.Validate(), ErrInvalidOrder)
}

func TestModifyOrderRequiresIdentityAndAChangedField(t *testing.T) {
	modify := ModifyOrder{
		AccountID:     "acct",
		InstrumentID:  MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-1",
		Price:         decimal.RequireFromString("100.01"),
	}
	require.NoError(t, modify.Validate())

	modify.Price = decimal.Zero
	require.ErrorIs(t, modify.Validate(), ErrInvalidOrder)

	modify.Price = decimal.RequireFromString("100.01")
	modify.ClientOrderID = ""
	require.ErrorIs(t, modify.Validate(), ErrInvalidOrder)
}

func TestBatchCancelAndCancelAllValidateCommandShape(t *testing.T) {
	instID := MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	batch := BatchCancelOrders{
		AccountID:    "acct",
		InstrumentID: instID,
		Cancels: []CancelOrder{{
			AccountID:     "acct",
			InstrumentID:  instID,
			ClientOrderID: "client-1",
		}},
	}
	require.NoError(t, batch.Validate())

	batch.Cancels = nil
	require.ErrorIs(t, batch.Validate(), ErrInvalidOrder)

	cancelAll := CancelAllOrders{AccountID: "acct", InstrumentID: instID}
	require.NoError(t, cancelAll.Validate())

	cancelAll.OrderSide = OrderSideBuy
	require.NoError(t, cancelAll.Validate())

	cancelAll.OrderSide = OrderSide("sideways")
	require.ErrorIs(t, cancelAll.Validate(), ErrInvalidOrder)

	cancelAll.AccountID = ""
	require.ErrorIs(t, cancelAll.Validate(), ErrInvalidOrder)
}

func TestQueryOrderRequiresAccountInstrumentAndOrderIdentity(t *testing.T) {
	query := QueryOrder{
		AccountID:     "acct",
		InstrumentID:  MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-1",
	}
	require.NoError(t, query.Validate())

	query.ClientOrderID = ""
	require.ErrorIs(t, query.Validate(), ErrInvalidOrder)

	query.OrderID = "order-1"
	require.NoError(t, query.Validate())
}

func TestQueryAccountRequiresAccountID(t *testing.T) {
	query := QueryAccount{AccountID: "acct"}
	require.NoError(t, query.Validate())

	query.AccountID = ""
	require.ErrorIs(t, query.Validate(), ErrInvalidAccount)
}

func TestCommandMetadataCanBeAttachedToOrderFactoryCommands(t *testing.T) {
	inst := MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	params := map[string]string{"source": "test"}
	factory := NewOrderFactory("acct", WithOrderMetadata(CommandMetadata{
		TraderID:      "trader-1",
		StrategyID:    "strategy-1",
		CommandID:     "command-1",
		CorrelationID: "corr-1",
		ClientID:      "client-1",
		Params:        params,
	}))
	params["source"] = "mutated"

	order := factory.Limit(inst, OrderSideBuy, decimal.RequireFromString("1"), decimal.RequireFromString("100"))
	order.Metadata.Params["source"] = "order-mutated"
	next := factory.Limit(inst, OrderSideBuy, decimal.RequireFromString("1"), decimal.RequireFromString("100"))
	overrideParams := map[string]string{"source": "override"}
	override := factory.Limit(inst, OrderSideBuy, decimal.RequireFromString("1"), decimal.RequireFromString("100"), WithCommandMetadata(CommandMetadata{
		CommandID: "command-2",
		Params:    overrideParams,
	}))
	overrideParams["source"] = "override-mutated"

	require.Equal(t, TraderID("trader-1"), order.Metadata.TraderID)
	require.Equal(t, StrategyID("strategy-1"), order.Metadata.StrategyID)
	require.Equal(t, CommandID("command-1"), order.Metadata.CommandID)
	require.Equal(t, CorrelationID("corr-1"), order.Metadata.CorrelationID)
	require.Equal(t, ExecutionClientID("client-1"), order.Metadata.ClientID)
	require.Equal(t, "order-mutated", order.Metadata.Params["source"])
	require.Equal(t, "test", next.Metadata.Params["source"])
	require.Equal(t, "override", override.Metadata.Params["source"])
}

func TestOrderListAppliesCommandMetadataDefaultsToChildren(t *testing.T) {
	inst := MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	list := OrderList{
		Metadata: CommandMetadata{
			CommandID: "list-command",
			Params:    map[string]string{"scope": "list"},
		},
		ID: "list-1",
		Orders: []SubmitOrder{
			{
				AccountID:     "acct",
				InstrumentID:  inst,
				OrderListID:   "list-1",
				ClientOrderID: "child-1",
				Side:          OrderSideBuy,
				Type:          OrderTypeLimit,
				TimeInForce:   TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("100"),
			},
			{
				Metadata:      CommandMetadata{CommandID: "child-command"},
				AccountID:     "acct",
				InstrumentID:  inst,
				OrderListID:   "list-1",
				ClientOrderID: "child-2",
				Side:          OrderSideSell,
				Type:          OrderTypeLimit,
				TimeInForce:   TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("101"),
			},
		},
	}

	withDefaults := list.WithCommandMetadataDefaults()
	withDefaults.Orders[0].Metadata.Params["scope"] = "mutated"

	require.Equal(t, CommandID("list-command"), withDefaults.Orders[0].Metadata.CommandID)
	require.Equal(t, CommandID("child-command"), withDefaults.Orders[1].Metadata.CommandID)
	require.Equal(t, "list", withDefaults.Orders[1].Metadata.Params["scope"])
	require.Empty(t, list.Orders[0].Metadata.CommandID)
}

func TestOrderListClassifiesBracketAndUniformInstrument(t *testing.T) {
	inst := MustInstrumentID("BTC-USDT-PERP.BINANCE")
	list := NewOrderFactory("acct").Bracket(BracketOrderRequest{
		InstrumentID: inst,
		Side:         OrderSideBuy,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("101"),
		TakeProfit:   decimal.RequireFromString("103"),
		StopLoss:     decimal.RequireFromString("99"),
	})

	require.NoError(t, list.Validate())
	require.True(t, list.IsBracket())
	require.Equal(t, OrderListKindBracket, list.Kind())

	uniform, ok := list.UniformInstrument()
	require.True(t, ok)
	require.Equal(t, inst, uniform)

	venue, ok := list.Venue()
	require.True(t, ok)
	require.Equal(t, Venue("BINANCE"), venue)
}

func TestOrderListAllowsMultiInstrumentSameVenueWithoutUniformInstrument(t *testing.T) {
	btc := MustInstrumentID("BTC-USDT-PERP.BINANCE")
	eth := MustInstrumentID("ETH-USDT-PERP.BINANCE")
	list := OrderList{
		ID: "list-multi-instrument",
		Orders: []SubmitOrder{
			{
				AccountID:     "acct",
				InstrumentID:  btc,
				OrderListID:   "list-multi-instrument",
				ClientOrderID: "btc-entry",
				Side:          OrderSideBuy,
				Type:          OrderTypeLimit,
				TimeInForce:   TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("101"),
			},
			{
				AccountID:     "acct",
				InstrumentID:  eth,
				OrderListID:   "list-multi-instrument",
				ClientOrderID: "eth-entry",
				Side:          OrderSideBuy,
				Type:          OrderTypeLimit,
				TimeInForce:   TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("101"),
			},
		},
	}

	require.NoError(t, list.Validate())
	_, ok := list.UniformInstrument()
	require.False(t, ok)

	venue, ok := list.Venue()
	require.True(t, ok)
	require.Equal(t, Venue("BINANCE"), venue)

	list.Orders[1].InstrumentID = MustInstrumentID("ETH-USDT-PERP.OKX")
	require.ErrorIs(t, list.Validate(), ErrInvalidOrder)
}

func TestAccountSnapshotValidatesBalancesAndMargins(t *testing.T) {
	snapshot := AccountSnapshot{
		AccountID: "acct",
		Venue:     "BINANCE",
		Type:      AccountTypeMargin,
		Balances: []Balance{{
			Currency: "USDT",
			Free:     "90",
			Locked:   "10",
			Total:    "100",
		}},
		Margins: []MarginBalance{{
			Currency:     "USDT",
			InstrumentID: MustInstrumentID("BTC-USDT-PERP.BINANCE"),
			Initial:      "12.5",
			Maintenance:  "6.25",
		}},
	}
	require.NoError(t, snapshot.Validate())

	snapshot.Balances[0].Total = "99"
	require.ErrorIs(t, snapshot.Validate(), ErrInvalidAccount)

	snapshot.Balances[0].Total = "100"
	snapshot.Margins[0].Initial = "-1"
	require.ErrorIs(t, snapshot.Validate(), ErrInvalidAccount)
}

func TestExecutionEventValidateRequiresSinglePayload(t *testing.T) {
	report := OrderStatusReport{
		AccountID:    AccountID("acct"),
		InstrumentID: MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderID:      OrderID("1"),
		Status:       OrderStatusAccepted,
	}
	require.NoError(t, report.Validate())
	require.NoError(t, ExecutionEvent{Order: &report}.Validate())
	require.ErrorIs(t, ExecutionEvent{}.Validate(), ErrInvalidExecutionEvent)
	require.ErrorIs(t, ExecutionEvent{Order: &report, Account: &AccountSnapshot{AccountID: "acct"}}.Validate(), ErrInvalidExecutionEvent)
}

func TestExecutionEventValidatesFillAndPositionPayloads(t *testing.T) {
	fill := FillReport{
		AccountID:    AccountID("acct"),
		InstrumentID: MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderID:      OrderID("order-1"),
		TradeID:      TradeID("trade-1"),
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("0.25"),
	}
	require.NoError(t, ExecutionEvent{Fill: &fill}.Validate())

	fill.Quantity = decimal.Zero
	require.ErrorIs(t, ExecutionEvent{Fill: &fill}.Validate(), ErrInvalidOrder)

	position := PositionStatusReport{
		AccountID:    AccountID("acct"),
		InstrumentID: MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		PositionID:   PositionID("BTC-USDT-SPOT.BINANCE"),
		Quantity:     decimal.RequireFromString("0.25"),
		EntryPrice:   decimal.RequireFromString("100"),
	}
	require.NoError(t, ExecutionEvent{Position: &position}.Validate())

	position.PositionID = ""
	require.ErrorIs(t, ExecutionEvent{Position: &position}.Validate(), ErrInvalidOrder)
}

func TestFillReportDetectsLegFillsAndAllowsMissingOrderID(t *testing.T) {
	fill := FillReport{
		AccountID:     AccountID("acct"),
		InstrumentID:  MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: ClientOrderID("spread-LEG-BTC"),
		TradeID:       TradeID("trade-leg-1"),
		Price:         decimal.RequireFromString("100"),
		Quantity:      decimal.RequireFromString("0.25"),
	}
	require.True(t, fill.IsLegFill())
	require.NoError(t, fill.Validate())

	fill.ClientOrderID = "plain-client"
	fill.VenueOrderID = "plain-venue"
	require.False(t, fill.IsLegFill())
	require.ErrorIs(t, fill.Validate(), ErrInvalidOrder)

	fill.IsLeg = true
	require.True(t, fill.IsLegFill())
	require.NoError(t, fill.Validate())
}

func TestOrderBookValidateKeepsInstrumentIdentity(t *testing.T) {
	book := OrderBook{
		InstrumentID: MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		Bids: []OrderBookLevel{{
			Price: decimal.RequireFromString("100"),
			Size:  decimal.RequireFromString("1"),
		}},
		Asks: []OrderBookLevel{{
			Price: decimal.RequireFromString("101"),
			Size:  decimal.RequireFromString("2"),
		}},
	}
	require.NoError(t, book.Validate())

	book.Asks[0].Price = decimal.RequireFromString("99")
	require.ErrorIs(t, book.Validate(), ErrInvalidMarketData)
}

func TestTradeTickValidatesNautilusStyleFieldsAndMarketEventIdentity(t *testing.T) {
	tick := TradeTick{
		InstrumentID:  MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		Price:         decimal.RequireFromString("100.5"),
		Size:          decimal.RequireFromString("0.25"),
		AggressorSide: AggressorSideBuyer,
		TradeID:       TradeID("venue-trade-1"),
		Timestamp:     testNow,
		InitTime:      testNow.Add(time.Millisecond),
	}
	require.NoError(t, tick.Validate())

	event := MarketEvent{Trade: &tick}
	require.NoError(t, event.Validate())
	require.Equal(t, tick.InstrumentID, event.InstrumentID())

	tick.Size = decimal.Zero
	require.ErrorIs(t, tick.Validate(), ErrInvalidMarketData)

	tick.Size = decimal.RequireFromString("0.25")
	tick.AggressorSide = AggressorSide("maker")
	require.ErrorIs(t, tick.Validate(), ErrInvalidMarketData)
}

func TestQuoteTickValidatesNautilusStyleTopOfBookAndMarketEventIdentity(t *testing.T) {
	quote := QuoteTick{
		InstrumentID: MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1.5"),
		AskSize:      decimal.RequireFromString("2.5"),
		Timestamp:    testNow,
		InitTime:     testNow.Add(time.Millisecond),
	}
	require.NoError(t, quote.Validate())

	event := MarketEvent{Quote: &quote}
	require.NoError(t, event.Validate())
	require.Equal(t, quote.InstrumentID, event.InstrumentID())

	quote.AskPrice = decimal.RequireFromString("99")
	require.ErrorIs(t, quote.Validate(), ErrInvalidMarketData)

	quote.AskPrice = decimal.RequireFromString("101")
	quote.BidSize = decimal.Zero
	require.ErrorIs(t, quote.Validate(), ErrInvalidMarketData)
}

func TestBarValidatesNautilusStyleOHLCVAndMarketEventIdentity(t *testing.T) {
	barType := NewTimeBarType(MustInstrumentID("BTC-USDT-SPOT.BINANCE"), time.Minute)
	bar := Bar{
		BarType:   barType,
		Open:      decimal.RequireFromString("100"),
		High:      decimal.RequireFromString("102"),
		Low:       decimal.RequireFromString("99"),
		Close:     decimal.RequireFromString("101"),
		Volume:    decimal.RequireFromString("12.5"),
		Timestamp: testNow,
		InitTime:  testNow.Add(time.Millisecond),
	}
	require.NoError(t, bar.Validate())

	event := MarketEvent{Bar: &bar}
	require.NoError(t, event.Validate())
	require.Equal(t, barType.InstrumentID, event.InstrumentID())

	bar.High = decimal.RequireFromString("98")
	require.ErrorIs(t, bar.Validate(), ErrInvalidMarketData)

	bar.High = decimal.RequireFromString("102")
	bar.Volume = decimal.RequireFromString("-1")
	require.ErrorIs(t, bar.Validate(), ErrInvalidMarketData)
}

func TestSubscribeMarketDataValidatesTypeAndDepth(t *testing.T) {
	sub := SubscribeMarketData{
		InstrumentID: MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		Type:         MarketDataTypeTicker,
	}
	require.NoError(t, sub.Validate())

	sub.Type = MarketDataTypeOrderBook
	sub.Depth = 10
	require.NoError(t, sub.Validate())

	sub.Depth = 0
	require.ErrorIs(t, sub.Validate(), ErrInvalidMarketData)

	sub.Type = MarketDataTypeTradeTick
	sub.Depth = 0
	require.NoError(t, sub.Validate())

	sub.Type = MarketDataTypeQuoteTick
	require.NoError(t, sub.Validate())

	sub.Type = MarketDataTypeBar
	sub.BarType = NewTimeBarType(sub.InstrumentID, time.Minute)
	require.NoError(t, sub.Validate())

	sub.BarType.InstrumentID = MustInstrumentID("ETH-USDT-SPOT.BINANCE")
	require.ErrorIs(t, sub.Validate(), ErrInvalidMarketData)

	sub.Type = MarketDataType("funding")
	require.ErrorIs(t, sub.Validate(), ErrInvalidMarketData)
}

var testNow = time.Unix(1000, 0)
