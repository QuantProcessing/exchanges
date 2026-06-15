package testsuite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/shopspring/decimal"
)

type PortfolioTesterConfig struct {
	Portfolio         *portfolio.Portfolio
	Cache             *cache.Cache
	AccountID         model.AccountID
	InstrumentID      model.InstrumentID
	ShortInstrumentID model.InstrumentID
	XRateInstrumentID model.InstrumentID
	MarkPrice         decimal.Decimal
}

type PortfolioTester struct {
	cfg PortfolioTesterConfig
}

func NewPortfolioTester(cfg PortfolioTesterConfig) *PortfolioTester {
	return &PortfolioTester{cfg: cfg}
}

func (p *PortfolioTester) Run(ctx context.Context, t *testing.T) ContractReport {
	t.Helper()
	_ = ctx
	return runContractCases(t, "portfolio", []contractCase{
		{id: "TC-P01", name: "Apply fills and position", run: func() error {
			if p.cfg.Portfolio == nil || p.cfg.Cache == nil {
				return fmt.Errorf("portfolio and cache are required")
			}
			if err := p.applyReferenceFills(); err != nil {
				return err
			}
			position, ok := p.cfg.Cache.PositionByInstrument(p.cfg.AccountID, p.cfg.InstrumentID)
			if !ok {
				return fmt.Errorf("position not found")
			}
			if position.Side != model.PositionSideLong || !position.Quantity.Equal(decimal.RequireFromString("0.6")) {
				return fmt.Errorf("unexpected position: %#v", position)
			}
			return nil
		}},
		{id: "TC-P02", name: "Realized and unrealized PnL", run: func() error {
			p.cfg.Portfolio.SetMark(p.cfg.AccountID, p.cfg.InstrumentID, p.cfg.MarkPrice)
			if got := p.cfg.Portfolio.RealizedPnL(p.cfg.AccountID, p.cfg.InstrumentID); !got.Equal(decimal.RequireFromString("4")) {
				return fmt.Errorf("realized pnl mismatch: %s", got)
			}
			if got := p.cfg.Portfolio.UnrealizedPnL(p.cfg.AccountID, p.cfg.InstrumentID); !got.Equal(decimal.RequireFromString("12")) {
				return fmt.Errorf("unrealized pnl mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P03", name: "Commission by currency", run: func() error {
			if got := p.cfg.Portfolio.Commission(p.cfg.AccountID, "USDT"); !got.Equal(decimal.RequireFromString("0.14")) {
				return fmt.Errorf("commission mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P04", name: "Market data mark update", run: func() error {
			if err := p.cfg.Portfolio.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
				InstrumentID: p.cfg.InstrumentID,
				BidPrice:     decimal.RequireFromString("125"),
				AskPrice:     decimal.RequireFromString("126"),
				BidSize:      decimal.RequireFromString("1"),
				AskSize:      decimal.RequireFromString("1"),
			}}); err != nil {
				return err
			}
			if got := p.cfg.Portfolio.UnrealizedPnL(p.cfg.AccountID, p.cfg.InstrumentID); !got.Equal(decimal.RequireFromString("15")) {
				return fmt.Errorf("market-data unrealized pnl mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P05", name: "Signed mark values", run: func() error {
			if p.cfg.ShortInstrumentID.String() == "" {
				return fmt.Errorf("short instrument is required")
			}
			if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
				AccountID:    p.cfg.AccountID,
				InstrumentID: p.cfg.ShortInstrumentID,
				OrderID:      "tc-p-short",
				TradeID:      "tc-p-trade-3",
				Side:         model.OrderSideSell,
				Price:        decimal.RequireFromString("50"),
				Quantity:     decimal.RequireFromString("2"),
			}); err != nil {
				return err
			}
			if err := p.cfg.Portfolio.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
				InstrumentID: p.cfg.ShortInstrumentID,
				BidPrice:     decimal.RequireFromString("45"),
				AskPrice:     decimal.RequireFromString("46"),
				BidSize:      decimal.RequireFromString("1"),
				AskSize:      decimal.RequireFromString("1"),
			}}); err != nil {
				return err
			}
			values := p.cfg.Portfolio.MarkValues(p.cfg.AccountID)
			if got := p.cfg.Portfolio.MarkValue(p.cfg.AccountID, p.cfg.InstrumentID); !got.Equal(decimal.RequireFromString("75")) {
				return fmt.Errorf("long mark value mismatch: %s", got)
			}
			if got := p.cfg.Portfolio.MarkValue(p.cfg.AccountID, p.cfg.ShortInstrumentID); !got.Equal(decimal.RequireFromString("-92")) {
				return fmt.Errorf("short mark value mismatch: %s", got)
			}
			if got := values["USDT"]; !got.Equal(decimal.RequireFromString("-17")) {
				return fmt.Errorf("aggregate mark value mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P06", name: "Account equity and margins", run: func() error {
			if err := p.cfg.Portfolio.UpdateAccount(model.AccountSnapshot{
				AccountID: p.cfg.AccountID,
				Venue:     p.cfg.InstrumentID.Venue,
				Type:      model.AccountTypeMargin,
				Balances: []model.Balance{{
					Currency: "USDT",
					Free:     "900",
					Locked:   "100",
					Total:    "1000",
				}},
				Margins: []model.MarginBalance{{
					Currency:     "USDT",
					InstrumentID: p.cfg.InstrumentID,
					Initial:      "125",
					Maintenance:  "75",
				}},
			}); err != nil {
				return err
			}
			p.cfg.Portfolio.SetMark(p.cfg.AccountID, p.cfg.InstrumentID, decimal.RequireFromString("120"))
			if got := p.cfg.Portfolio.BalancesLocked(p.cfg.AccountID)["USDT"]; !got.Equal(decimal.RequireFromString("100")) {
				return fmt.Errorf("locked balance mismatch: %s", got)
			}
			if got := p.cfg.Portfolio.MarginsInit(p.cfg.AccountID)[p.cfg.InstrumentID]; !got.Equal(decimal.RequireFromString("125")) {
				return fmt.Errorf("initial margin mismatch: %s", got)
			}
			if got := p.cfg.Portfolio.MarginsMaint(p.cfg.AccountID)[p.cfg.InstrumentID]; !got.Equal(decimal.RequireFromString("75")) {
				return fmt.Errorf("maintenance margin mismatch: %s", got)
			}
			if got := p.cfg.Portfolio.Equity(p.cfg.AccountID)["USDT"]; !got.Equal(decimal.RequireFromString("1020")) {
				return fmt.Errorf("equity mismatch: %s", got)
			}
			if got := p.cfg.Portfolio.AvailableEquity(p.cfg.AccountID)["USDT"]; !got.Equal(decimal.RequireFromString("795")) {
				return fmt.Errorf("available equity mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P07", name: "Account base currency conversion", run: func() error {
			if p.cfg.XRateInstrumentID == (model.InstrumentID{}) {
				return fmt.Errorf("xrate instrument is required")
			}
			if err := p.cfg.Portfolio.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
				InstrumentID: p.cfg.XRateInstrumentID,
				BidPrice:     decimal.RequireFromString("1.10"),
				AskPrice:     decimal.RequireFromString("1.10"),
				BidSize:      decimal.RequireFromString("1000"),
				AskSize:      decimal.RequireFromString("1000"),
			}}); err != nil {
				return err
			}
			if err := p.cfg.Portfolio.UpdateAccount(model.AccountSnapshot{
				AccountID:    p.cfg.AccountID,
				Venue:        p.cfg.InstrumentID.Venue,
				Type:         model.AccountTypeMargin,
				BaseCurrency: "USD",
				Balances: []model.Balance{{
					Currency: "USDT",
					Free:     "900",
					Locked:   "100",
					Total:    "1000",
				}},
				Margins: []model.MarginBalance{{
					Currency:     "USDT",
					InstrumentID: p.cfg.InstrumentID,
					Initial:      "125",
					Maintenance:  "75",
				}},
			}); err != nil {
				return err
			}
			equity := p.cfg.Portfolio.Equity(p.cfg.AccountID)
			if len(equity) != 1 {
				return fmt.Errorf("converted equity should have one currency: %#v", equity)
			}
			if got := equity["USD"]; !got.Equal(decimal.RequireFromString("1122")) {
				return fmt.Errorf("converted equity mismatch: %s", got)
			}
			available := p.cfg.Portfolio.AvailableEquity(p.cfg.AccountID)
			if len(available) != 1 {
				return fmt.Errorf("converted available equity should have one currency: %#v", available)
			}
			if got := available["USD"]; !got.Equal(decimal.RequireFromString("874.5")) {
				return fmt.Errorf("converted available equity mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P08", name: "Event-driven execution updates", run: func() error {
			accountID := model.AccountID(fmt.Sprintf("%s-event", p.cfg.AccountID))
			account := model.AccountSnapshot{
				AccountID: accountID,
				Venue:     p.cfg.InstrumentID.Venue,
				Type:      model.AccountTypeMargin,
				Balances: []model.Balance{{
					Currency: "USDT",
					Free:     "1000",
					Total:    "1000",
				}},
			}
			if err := p.cfg.Portfolio.HandleExecutionEvent(model.ExecutionEvent{Account: &account}); err != nil {
				return err
			}
			if _, ok := p.cfg.Cache.Account(accountID); !ok {
				return fmt.Errorf("event account was not cached")
			}
			order := model.OrderStatusReport{
				Metadata:       model.CommandMetadata{StrategyID: "tc-p-event-strategy"},
				AccountID:      accountID,
				InstrumentID:   p.cfg.InstrumentID,
				OrderID:        "tc-p-event-order",
				ClientOrderID:  "tc-p-event-client",
				Status:         model.OrderStatusAccepted,
				Side:           model.OrderSideBuy,
				Type:           model.OrderTypeLimit,
				Quantity:       decimal.RequireFromString("1"),
				LeavesQuantity: decimal.RequireFromString("1"),
				Price:          decimal.RequireFromString("100"),
			}
			if err := p.cfg.Portfolio.HandleExecutionEvent(model.ExecutionEvent{Order: &order}); err != nil {
				return err
			}
			cachedOrder, ok := p.cfg.Cache.OrderByClientID(accountID, "tc-p-event-client")
			if !ok {
				return fmt.Errorf("event order was not indexed by client id")
			}
			if cachedOrder.Metadata.StrategyID != "tc-p-event-strategy" {
				return fmt.Errorf("event order metadata mismatch: %#v", cachedOrder.Metadata)
			}
			fill := model.FillReport{
				AccountID:    accountID,
				InstrumentID: p.cfg.InstrumentID,
				OrderID:      order.OrderID,
				TradeID:      "tc-p-event-trade",
				Side:         model.OrderSideBuy,
				Price:        decimal.RequireFromString("100"),
				Quantity:     decimal.RequireFromString("1"),
				Fee:          decimal.RequireFromString("0.02"),
				FeeCurrency:  "USDT",
			}
			if err := p.cfg.Portfolio.HandleExecutionEvent(model.ExecutionEvent{Fill: &fill}); err != nil {
				return err
			}
			position, ok := p.cfg.Cache.PositionByInstrument(accountID, p.cfg.InstrumentID)
			if !ok {
				return fmt.Errorf("event fill did not create a position")
			}
			if position.Side != model.PositionSideLong || !position.Quantity.Equal(decimal.RequireFromString("1")) {
				return fmt.Errorf("event fill position mismatch: %#v", position)
			}
			if got := p.cfg.Portfolio.Commission(accountID, "USDT"); !got.Equal(decimal.RequireFromString("0.02")) {
				return fmt.Errorf("event fill commission mismatch: %s", got)
			}
			reportedPosition := model.PositionStatusReport{
				Metadata:        model.CommandMetadata{StrategyID: "tc-p-event-strategy"},
				AccountID:       accountID,
				InstrumentID:    p.cfg.InstrumentID,
				PositionID:      "tc-p-event-position",
				VenuePositionID: "tc-p-event-venue-position",
				Side:            model.PositionSideLong,
				Quantity:        decimal.RequireFromString("0.5"),
				EntryPrice:      decimal.RequireFromString("101"),
			}
			if err := p.cfg.Portfolio.HandleExecutionEvent(model.ExecutionEvent{Position: &reportedPosition}); err != nil {
				return err
			}
			position, ok = p.cfg.Cache.PositionByInstrument(accountID, p.cfg.InstrumentID)
			if !ok {
				return fmt.Errorf("event position report was not indexed by instrument")
			}
			if position.PositionID != reportedPosition.PositionID || !position.Quantity.Equal(decimal.RequireFromString("0.5")) {
				return fmt.Errorf("event position report mismatch: %#v", position)
			}
			return nil
		}},
		{id: "TC-P09", name: "Cash and margin accounting paths", run: func() error {
			cashAccountID := model.AccountID(fmt.Sprintf("%s-cash", p.cfg.AccountID))
			marginAccountID := model.AccountID(fmt.Sprintf("%s-margin", p.cfg.AccountID))
			if err := p.cfg.Portfolio.UpdateAccount(model.AccountSnapshot{
				AccountID: cashAccountID,
				Venue:     p.cfg.InstrumentID.Venue,
				Type:      model.AccountTypeCash,
				Balances: []model.Balance{{
					Currency: "USDT",
					Free:     "900",
					Total:    "900",
				}},
			}); err != nil {
				return err
			}
			if err := p.cfg.Portfolio.UpdateAccount(model.AccountSnapshot{
				AccountID: marginAccountID,
				Venue:     p.cfg.InstrumentID.Venue,
				Type:      model.AccountTypeMargin,
				Balances: []model.Balance{{
					Currency: "USDT",
					Free:     "900",
					Total:    "900",
				}},
				Margins: []model.MarginBalance{{
					Currency:     "USDT",
					InstrumentID: p.cfg.InstrumentID,
					Initial:      "25",
				}},
			}); err != nil {
				return err
			}
			for _, accountID := range []model.AccountID{cashAccountID, marginAccountID} {
				if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
					AccountID:    accountID,
					InstrumentID: p.cfg.InstrumentID,
					OrderID:      model.OrderID(fmt.Sprintf("tc-p-%s-buy", accountID)),
					TradeID:      model.TradeID(fmt.Sprintf("tc-p-%s-trade", accountID)),
					Side:         model.OrderSideBuy,
					Price:        decimal.RequireFromString("100"),
					Quantity:     decimal.RequireFromString("1"),
				}); err != nil {
					return err
				}
				p.cfg.Portfolio.SetMark(accountID, p.cfg.InstrumentID, decimal.RequireFromString("120"))
			}
			if got := p.cfg.Portfolio.Equity(cashAccountID)["USDT"]; !got.Equal(decimal.RequireFromString("1020")) {
				return fmt.Errorf("cash equity mismatch: %s", got)
			}
			if got := p.cfg.Portfolio.AvailableEquity(cashAccountID)["USDT"]; !got.Equal(decimal.RequireFromString("1020")) {
				return fmt.Errorf("cash available equity mismatch: %s", got)
			}
			if got := p.cfg.Portfolio.Equity(marginAccountID)["USDT"]; !got.Equal(decimal.RequireFromString("920")) {
				return fmt.Errorf("margin equity mismatch: %s", got)
			}
			if got := p.cfg.Portfolio.AvailableEquity(marginAccountID)["USDT"]; !got.Equal(decimal.RequireFromString("895")) {
				return fmt.Errorf("margin available equity mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P10", name: "Fill balance deltas", run: func() error {
			accountID := model.AccountID(fmt.Sprintf("%s-balance", p.cfg.AccountID))
			if err := p.cfg.Portfolio.UpdateAccount(model.AccountSnapshot{
				AccountID: accountID,
				Venue:     p.cfg.InstrumentID.Venue,
				Type:      model.AccountTypeMargin,
				Balances: []model.Balance{{
					Currency: "USDT",
					Free:     "1000",
					Total:    "1000",
				}},
			}); err != nil {
				return err
			}
			open := model.FillReport{
				AccountID:    accountID,
				InstrumentID: p.cfg.InstrumentID,
				OrderID:      "tc-p-balance-buy",
				TradeID:      "tc-p-balance-trade-1",
				Side:         model.OrderSideBuy,
				Price:        decimal.RequireFromString("100"),
				Quantity:     decimal.RequireFromString("1"),
				Fee:          decimal.RequireFromString("0.10"),
				FeeCurrency:  "USDT",
			}
			if err := p.cfg.Portfolio.ApplyFill(open); err != nil {
				return err
			}
			account, ok := p.cfg.Cache.Account(accountID)
			if !ok {
				return fmt.Errorf("balance account not cached")
			}
			if account.Balances[0].Total != "999.9" || account.Balances[0].Free != "999.9" {
				return fmt.Errorf("open fill balance mismatch: %#v", account.Balances[0])
			}
			close := model.FillReport{
				AccountID:    accountID,
				InstrumentID: p.cfg.InstrumentID,
				OrderID:      "tc-p-balance-sell",
				TradeID:      "tc-p-balance-trade-2",
				Side:         model.OrderSideSell,
				Price:        decimal.RequireFromString("110"),
				Quantity:     decimal.RequireFromString("0.4"),
				Fee:          decimal.RequireFromString("0.04"),
				FeeCurrency:  "USDT",
			}
			if err := p.cfg.Portfolio.ApplyFill(close); err != nil {
				return err
			}
			account, ok = p.cfg.Cache.Account(accountID)
			if !ok {
				return fmt.Errorf("balance account not cached after close")
			}
			if account.Balances[0].Total != "1003.86" || account.Balances[0].Free != "1003.86" {
				return fmt.Errorf("close fill balance mismatch: %#v", account.Balances[0])
			}
			if err := p.cfg.Portfolio.ApplyFill(close); err != nil {
				return err
			}
			account, ok = p.cfg.Cache.Account(accountID)
			if !ok {
				return fmt.Errorf("balance account not cached after duplicate")
			}
			if account.Balances[0].Total != "1003.86" || account.Balances[0].Free != "1003.86" {
				return fmt.Errorf("duplicate fill changed balance: %#v", account.Balances[0])
			}
			return nil
		}},
		{id: "TC-P11", name: "Unrealized PnL selected prices and marks", run: func() error {
			accountID := model.AccountID(fmt.Sprintf("%s-mark", p.cfg.AccountID))
			if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
				AccountID:    accountID,
				InstrumentID: p.cfg.InstrumentID,
				OrderID:      "tc-p-mark-buy",
				TradeID:      "tc-p-mark-trade-1",
				Side:         model.OrderSideBuy,
				Price:        decimal.RequireFromString("100"),
				Quantity:     decimal.RequireFromString("1"),
			}); err != nil {
				return err
			}
			p.cfg.Portfolio.SetMark(accountID, p.cfg.InstrumentID, decimal.RequireFromString("105"))
			if got := p.cfg.Portfolio.UnrealizedPnL(accountID, p.cfg.InstrumentID); !got.Equal(decimal.RequireFromString("5")) {
				return fmt.Errorf("explicit mark unrealized pnl mismatch: %s", got)
			}
			if err := p.cfg.Portfolio.ApplyMarketEvent(model.MarketEvent{Trade: &model.TradeTick{
				InstrumentID:  p.cfg.InstrumentID,
				Price:         decimal.RequireFromString("107"),
				Size:          decimal.RequireFromString("1"),
				AggressorSide: model.AggressorSideBuyer,
				TradeID:       "tc-p-mark-market-trade",
			}}); err != nil {
				return err
			}
			if got := p.cfg.Portfolio.UnrealizedPnL(accountID, p.cfg.InstrumentID); !got.Equal(decimal.RequireFromString("7")) {
				return fmt.Errorf("trade mark unrealized pnl mismatch: %s", got)
			}
			if err := p.cfg.Portfolio.ApplyMarketEvent(model.MarketEvent{Ticker: &model.Ticker{
				InstrumentID: p.cfg.InstrumentID,
				Bid:          decimal.RequireFromString("107.5"),
				Ask:          decimal.RequireFromString("108.5"),
				Last:         decimal.RequireFromString("108"),
			}}); err != nil {
				return err
			}
			if got := p.cfg.Portfolio.UnrealizedPnL(accountID, p.cfg.InstrumentID); !got.Equal(decimal.RequireFromString("8")) {
				return fmt.Errorf("ticker mark unrealized pnl mismatch: %s", got)
			}
			if err := p.cfg.Portfolio.ApplyMarketEvent(model.MarketEvent{Bar: &model.Bar{
				BarType: model.NewTimeBarType(p.cfg.InstrumentID, time.Minute),
				Open:    decimal.RequireFromString("107"),
				High:    decimal.RequireFromString("110"),
				Low:     decimal.RequireFromString("106"),
				Close:   decimal.RequireFromString("109"),
				Volume:  decimal.RequireFromString("10"),
			}}); err != nil {
				return err
			}
			if got := p.cfg.Portfolio.UnrealizedPnL(accountID, p.cfg.InstrumentID); !got.Equal(decimal.RequireFromString("9")) {
				return fmt.Errorf("bar mark unrealized pnl mismatch: %s", got)
			}
			if err := p.cfg.Portfolio.ApplyMarketEvent(model.MarketEvent{OrderBook: &model.OrderBook{
				InstrumentID: p.cfg.InstrumentID,
				Bids: []model.OrderBookLevel{{
					Price: decimal.RequireFromString("110"),
					Size:  decimal.RequireFromString("1"),
				}},
				Asks: []model.OrderBookLevel{{
					Price: decimal.RequireFromString("111"),
					Size:  decimal.RequireFromString("1"),
				}},
			}}); err != nil {
				return err
			}
			if got := p.cfg.Portfolio.UnrealizedPnL(accountID, p.cfg.InstrumentID); !got.Equal(decimal.RequireFromString("10")) {
				return fmt.Errorf("book mark unrealized pnl mismatch: %s", got)
			}
			if err := p.cfg.Portfolio.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
				InstrumentID: p.cfg.InstrumentID,
				BidPrice:     decimal.RequireFromString("111"),
				AskPrice:     decimal.RequireFromString("112"),
				BidSize:      decimal.RequireFromString("1"),
				AskSize:      decimal.RequireFromString("1"),
			}}); err != nil {
				return err
			}
			if got := p.cfg.Portfolio.UnrealizedPnL(accountID, p.cfg.InstrumentID); !got.Equal(decimal.RequireFromString("11")) {
				return fmt.Errorf("quote mark unrealized pnl mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P12", name: "Net position and exposure aggregation", run: func() error {
			if p.cfg.XRateInstrumentID == (model.InstrumentID{}) {
				return fmt.Errorf("xrate instrument is required")
			}
			main := model.Instrument{
				ID:        model.MustInstrumentID("SOL-USDT-PERP.BINANCE"),
				RawSymbol: "SOLUSDT",
				Type:      model.InstrumentTypePerp,
				Base:      "SOL",
				Quote:     "USDT",
				Settle:    "USDT",
				PriceTick: decimal.RequireFromString("0.01"),
				SizeTick:  decimal.RequireFromString("0.001"),
				Status:    model.InstrumentStatusTrading,
			}
			other := model.Instrument{
				ID:        model.MustInstrumentID("ADA-USDT-PERP.OKX"),
				RawSymbol: "ADAUSDT",
				Type:      model.InstrumentTypePerp,
				Base:      "ADA",
				Quote:     "USDT",
				Settle:    "USDT",
				PriceTick: decimal.RequireFromString("0.01"),
				SizeTick:  decimal.RequireFromString("0.001"),
				Status:    model.InstrumentStatusTrading,
			}
			if err := p.cfg.Cache.PutInstrument(main); err != nil {
				return err
			}
			if err := p.cfg.Cache.PutInstrument(other); err != nil {
				return err
			}
			if err := p.cfg.Portfolio.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
				InstrumentID: p.cfg.XRateInstrumentID,
				BidPrice:     decimal.RequireFromString("1.10"),
				AskPrice:     decimal.RequireFromString("1.10"),
				BidSize:      decimal.RequireFromString("1000"),
				AskSize:      decimal.RequireFromString("1000"),
			}}); err != nil {
				return err
			}
			accountA := model.AccountID(fmt.Sprintf("%s-net-a", p.cfg.AccountID))
			accountB := model.AccountID(fmt.Sprintf("%s-net-b", p.cfg.AccountID))
			if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
				AccountID:    accountA,
				InstrumentID: main.ID,
				OrderID:      "tc-p-net-a-btc",
				TradeID:      "tc-p-net-a-btc-trade",
				Side:         model.OrderSideBuy,
				Price:        decimal.RequireFromString("100"),
				Quantity:     decimal.RequireFromString("1"),
			}); err != nil {
				return err
			}
			if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
				AccountID:    accountB,
				InstrumentID: main.ID,
				OrderID:      "tc-p-net-b-btc",
				TradeID:      "tc-p-net-b-btc-trade",
				Side:         model.OrderSideSell,
				Price:        decimal.RequireFromString("120"),
				Quantity:     decimal.RequireFromString("0.4"),
			}); err != nil {
				return err
			}
			if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
				AccountID:    accountA,
				InstrumentID: other.ID,
				OrderID:      "tc-p-net-a-eth",
				TradeID:      "tc-p-net-a-eth-trade",
				Side:         model.OrderSideBuy,
				Price:        decimal.RequireFromString("50"),
				Quantity:     decimal.RequireFromString("2"),
			}); err != nil {
				return err
			}
			p.cfg.Portfolio.SetMark(accountA, main.ID, decimal.RequireFromString("110"))
			p.cfg.Portfolio.SetMark(accountB, main.ID, decimal.RequireFromString("110"))
			p.cfg.Portfolio.SetMark(accountA, other.ID, decimal.RequireFromString("55"))
			if got := p.cfg.Portfolio.NetPosition("", main.ID); !got.Equal(decimal.RequireFromString("0.6")) {
				return fmt.Errorf("net position mismatch: %s", got)
			}
			byInstrument := p.cfg.Portfolio.NetExposuresByInstrument("", "USD")
			if got := byInstrument[main.ID]; !got.Equal(decimal.RequireFromString("72.6")) {
				return fmt.Errorf("net exposure by instrument mismatch: %s", got)
			}
			if got := byInstrument[other.ID]; !got.Equal(decimal.RequireFromString("121")) {
				return fmt.Errorf("other net exposure by instrument mismatch: %s", got)
			}
			byAccount := p.cfg.Portfolio.NetExposuresByAccount("USD")
			if got := byAccount[accountA]; !got.Equal(decimal.RequireFromString("242")) {
				return fmt.Errorf("net exposure by account mismatch: %s", got)
			}
			if got := byAccount[accountB]; !got.Equal(decimal.RequireFromString("-48.4")) {
				return fmt.Errorf("short net exposure by account mismatch: %s", got)
			}
			byVenue := p.cfg.Portfolio.NetExposuresByVenue("USD")
			if got := byVenue[main.ID.Venue]; !got.Equal(decimal.RequireFromString("72.6")) {
				return fmt.Errorf("net exposure by venue mismatch: %s", got)
			}
			if got := byVenue["OKX"]; !got.Equal(decimal.RequireFromString("121")) {
				return fmt.Errorf("other venue net exposure mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P13", name: "Explicit conversion hooks", run: func() error {
			inst := model.Instrument{
				ID:        model.MustInstrumentID("LINK-EUR-PERP.BINANCE"),
				RawSymbol: "LINKEUR",
				Type:      model.InstrumentTypePerp,
				Base:      "LINK",
				Quote:     "EUR",
				Settle:    "EUR",
				PriceTick: decimal.RequireFromString("0.01"),
				SizeTick:  decimal.RequireFromString("0.001"),
				Status:    model.InstrumentStatusTrading,
			}
			if err := p.cfg.Cache.PutInstrument(inst); err != nil {
				return err
			}
			if err := p.cfg.Portfolio.SetConversionRate("EUR", "USD", decimal.RequireFromString("1.20")); err != nil {
				return err
			}
			accountID := model.AccountID(fmt.Sprintf("%s-convert", p.cfg.AccountID))
			if err := p.cfg.Portfolio.UpdateAccount(model.AccountSnapshot{
				AccountID:    accountID,
				Venue:        inst.ID.Venue,
				Type:         model.AccountTypeMargin,
				BaseCurrency: "USD",
				Balances: []model.Balance{{
					Currency: "EUR",
					Free:     "100",
					Total:    "100",
				}},
			}); err != nil {
				return err
			}
			if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
				AccountID:    accountID,
				InstrumentID: inst.ID,
				OrderID:      "tc-p-convert-buy",
				TradeID:      "tc-p-convert-trade",
				Side:         model.OrderSideBuy,
				Price:        decimal.RequireFromString("100"),
				Quantity:     decimal.RequireFromString("1"),
			}); err != nil {
				return err
			}
			p.cfg.Portfolio.SetMark(accountID, inst.ID, decimal.RequireFromString("120"))
			if got := p.cfg.Portfolio.Exposure(accountID, "USD"); !got.Equal(decimal.RequireFromString("144")) {
				return fmt.Errorf("converted exposure mismatch: %s", got)
			}
			if got := p.cfg.Portfolio.Equity(accountID)["USD"]; !got.Equal(decimal.RequireFromString("144")) {
				return fmt.Errorf("converted equity mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P14", name: "PnL cache invalidation", run: func() error {
			inst := model.Instrument{
				ID:        model.MustInstrumentID("XRP-USDT-PERP.BINANCE"),
				RawSymbol: "XRPUSDT",
				Type:      model.InstrumentTypePerp,
				Base:      "XRP",
				Quote:     "USDT",
				Settle:    "USDT",
				PriceTick: decimal.RequireFromString("0.0001"),
				SizeTick:  decimal.RequireFromString("1"),
				Status:    model.InstrumentStatusTrading,
			}
			if err := p.cfg.Cache.PutInstrument(inst); err != nil {
				return err
			}
			accountID := model.AccountID(fmt.Sprintf("%s-pnl-cache", p.cfg.AccountID))
			if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
				AccountID:    accountID,
				InstrumentID: inst.ID,
				OrderID:      "tc-p-cache-buy",
				TradeID:      "tc-p-cache-trade",
				Side:         model.OrderSideBuy,
				Price:        decimal.RequireFromString("100"),
				Quantity:     decimal.RequireFromString("1"),
			}); err != nil {
				return err
			}
			p.cfg.Portfolio.SetMark(accountID, inst.ID, decimal.RequireFromString("110"))
			if got := p.cfg.Portfolio.UnrealizedPnL(accountID, inst.ID); !got.Equal(decimal.RequireFromString("10")) {
				return fmt.Errorf("initial cached pnl mismatch: %s", got)
			}
			if err := p.cfg.Portfolio.ApplyMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
				InstrumentID: inst.ID,
				BidPrice:     decimal.RequireFromString("120"),
				AskPrice:     decimal.RequireFromString("121"),
				BidSize:      decimal.RequireFromString("1"),
				AskSize:      decimal.RequireFromString("1"),
			}}); err != nil {
				return err
			}
			if got := p.cfg.Portfolio.UnrealizedPnL(accountID, inst.ID); !got.Equal(decimal.RequireFromString("20")) {
				return fmt.Errorf("market invalidated pnl mismatch: %s", got)
			}
			position := model.PositionStatusReport{
				AccountID:    accountID,
				InstrumentID: inst.ID,
				PositionID:   "tc-p-cache-position",
				Side:         model.PositionSideLong,
				Quantity:     decimal.RequireFromString("2"),
				EntryPrice:   decimal.RequireFromString("100"),
			}
			if err := p.cfg.Portfolio.HandleExecutionEvent(model.ExecutionEvent{Position: &position}); err != nil {
				return err
			}
			if got := p.cfg.Portfolio.UnrealizedPnL(accountID, inst.ID); !got.Equal(decimal.RequireFromString("40")) {
				return fmt.Errorf("position invalidated pnl mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-P15", name: "Analyzer closed-trade records", run: func() error {
			inst := model.Instrument{
				ID:        model.MustInstrumentID("DOGE-USDT-PERP.BINANCE"),
				RawSymbol: "DOGEUSDT",
				Type:      model.InstrumentTypePerp,
				Base:      "DOGE",
				Quote:     "USDT",
				Settle:    "USDT",
				PriceTick: decimal.RequireFromString("0.0001"),
				SizeTick:  decimal.RequireFromString("1"),
				Status:    model.InstrumentStatusTrading,
			}
			if err := p.cfg.Cache.PutInstrument(inst); err != nil {
				return err
			}
			records := make([]portfolio.TradeRecord, 0, 1)
			p.cfg.Portfolio.SetAnalyzer(portfolio.AnalyzerFunc(func(record portfolio.TradeRecord) {
				records = append(records, record)
			}))
			defer p.cfg.Portfolio.SetAnalyzer(nil)
			if err := p.cfg.Portfolio.SetConversionRate("USDT", "USD", decimal.RequireFromString("1.10")); err != nil {
				return err
			}
			accountID := model.AccountID(fmt.Sprintf("%s-analyzer", p.cfg.AccountID))
			if err := p.cfg.Portfolio.UpdateAccount(model.AccountSnapshot{
				AccountID:    accountID,
				Venue:        inst.ID.Venue,
				Type:         model.AccountTypeMargin,
				BaseCurrency: "USD",
				Balances: []model.Balance{{
					Currency: "USDT",
					Free:     "1000",
					Total:    "1000",
				}},
			}); err != nil {
				return err
			}
			if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
				AccountID:    accountID,
				InstrumentID: inst.ID,
				OrderID:      "tc-p-analyzer-buy",
				TradeID:      "tc-p-analyzer-trade-1",
				Side:         model.OrderSideBuy,
				Price:        decimal.RequireFromString("100"),
				Quantity:     decimal.RequireFromString("1"),
			}); err != nil {
				return err
			}
			if len(records) != 0 {
				return fmt.Errorf("analyzer should not record opening fill: %#v", records)
			}
			if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
				AccountID:    accountID,
				InstrumentID: inst.ID,
				OrderID:      "tc-p-analyzer-sell",
				TradeID:      "tc-p-analyzer-trade-2",
				Side:         model.OrderSideSell,
				Price:        decimal.RequireFromString("110"),
				Quantity:     decimal.RequireFromString("1"),
			}); err != nil {
				return err
			}
			if len(records) != 1 {
				return fmt.Errorf("expected one analyzer record, got %d", len(records))
			}
			record := records[0]
			if record.AccountID != accountID || record.InstrumentID != inst.ID {
				return fmt.Errorf("analyzer identity mismatch: %#v", record)
			}
			if record.Currency != "USDT" || !record.RealizedPnL.Equal(decimal.RequireFromString("10")) {
				return fmt.Errorf("analyzer realized pnl mismatch: %#v", record)
			}
			if record.AccountCurrency != "USD" || !record.AccountCurrencyPnL.Equal(decimal.RequireFromString("11")) {
				return fmt.Errorf("analyzer account pnl mismatch: %#v", record)
			}
			return nil
		}},
		{id: "TC-P16", name: "Leg fills use explicit position IDs", run: func() error {
			if p.cfg.Cache == nil || p.cfg.Portfolio == nil {
				return fmt.Errorf("portfolio and cache are required")
			}
			accountID := model.AccountID(fmt.Sprintf("%s-leg", p.cfg.AccountID))
			if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
				AccountID:     accountID,
				InstrumentID:  p.cfg.InstrumentID,
				VenueOrderID:  "tc-p-venue-LEG-1",
				ClientOrderID: "tc-p-client-LEG-1",
				TradeID:       "tc-p-trade-leg-1",
				PositionID:    "tc-p-leg-position-1",
				IsLeg:         true,
				Side:          model.OrderSideBuy,
				Price:         decimal.RequireFromString("100"),
				Quantity:      decimal.RequireFromString("0.25"),
			}); err != nil {
				return err
			}
			position, ok := p.cfg.Cache.Position(accountID, "tc-p-leg-position-1")
			if !ok {
				return fmt.Errorf("leg position not found")
			}
			if position.InstrumentID != p.cfg.InstrumentID || position.Side != model.PositionSideLong || !position.Quantity.Equal(decimal.RequireFromString("0.25")) {
				return fmt.Errorf("unexpected leg position: %+v", position)
			}
			return nil
		}},
	})
}

func (p *PortfolioTester) applyReferenceFills() error {
	if err := p.cfg.Portfolio.ApplyFill(model.FillReport{
		AccountID:    p.cfg.AccountID,
		InstrumentID: p.cfg.InstrumentID,
		OrderID:      "tc-p-buy",
		TradeID:      "tc-p-trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
		Fee:          decimal.RequireFromString("0.10"),
		FeeCurrency:  "USDT",
	}); err != nil {
		return err
	}
	return p.cfg.Portfolio.ApplyFill(model.FillReport{
		AccountID:    p.cfg.AccountID,
		InstrumentID: p.cfg.InstrumentID,
		OrderID:      "tc-p-sell",
		TradeID:      "tc-p-trade-2",
		Side:         model.OrderSideSell,
		Price:        decimal.RequireFromString("110"),
		Quantity:     decimal.RequireFromString("0.4"),
		Fee:          decimal.RequireFromString("0.04"),
		FeeCurrency:  "USDT",
	})
}
