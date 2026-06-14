package testsuite

import (
	"context"
	"fmt"
	"testing"

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
