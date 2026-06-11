package binance

import (
	"fmt"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

func spotAccountState(accountID model.AccountID, resp *spot.AccountResponse) (model.AccountState, error) {
	if resp == nil {
		return model.AccountState{}, fmt.Errorf("%w: nil spot account response", model.ErrInvalidAccountState)
	}
	state := model.AccountState{
		AccountID: accountID,
		Venue:     model.VenueBinance,
		Type:      model.AccountTypeCash,
		Reported:  true,
		EventTime: timeFromUnixMilli(resp.UpdateTime),
		InitTime:  time.Now(),
	}
	for _, b := range resp.Balances {
		free := model.Money{Amount: parseDecimal(b.Free), Currency: model.Currency(b.Asset)}
		locked := model.Money{Amount: parseDecimal(b.Locked), Currency: model.Currency(b.Asset)}
		total := model.Money{Amount: free.Amount.Add(locked.Amount), Currency: model.Currency(b.Asset)}
		if total.Amount.IsZero() {
			continue
		}
		bal, err := model.NewBalance(total, locked, free)
		if err != nil {
			return model.AccountState{}, err
		}
		state.Balances = append(state.Balances, bal)
	}
	return state, nil
}

func perpAccountState(accountID model.AccountID, resp *perp.AccountResponse) (model.AccountState, error) {
	if resp == nil {
		return model.AccountState{}, fmt.Errorf("%w: nil perp account response", model.ErrInvalidAccountState)
	}
	state := model.AccountState{
		AccountID: accountID,
		Venue:     model.VenueBinance,
		Type:      model.AccountTypeMargin,
		Reported:  true,
		EventTime: timeFromUnixMilli(resp.UpdateTime),
		InitTime:  time.Now(),
	}
	n := symbolNormalizer{}
	for _, a := range resp.Assets {
		ccy := model.Currency(a.Asset)
		total := model.Money{Amount: parseDecimal(a.MarginBalance), Currency: ccy}
		free := model.Money{Amount: parseDecimal(a.AvailableBalance), Currency: ccy}
		if !total.Amount.IsZero() || !free.Amount.IsZero() {
			bal, err := model.BalanceFromTotalAndFree(total, free)
			if err != nil {
				return model.AccountState{}, err
			}
			state.Balances = append(state.Balances, bal)
		}
		initial := model.Money{Amount: parseDecimal(a.InitialMargin), Currency: ccy}
		maintenance := model.Money{Amount: parseDecimal(a.MaintMargin), Currency: ccy}
		if !initial.Amount.IsZero() || !maintenance.Amount.IsZero() {
			state.Margins = append(state.Margins, model.MarginBalance{
				Initial:     initial,
				Maintenance: maintenance,
			})
		}
	}
	for _, p := range resp.Positions {
		qty := parseDecimal(p.PositionAmt)
		if qty.IsZero() {
			continue
		}
		id, err := n.ToInstrumentID(p.Symbol, venue.ProductHintPerp)
		if err != nil {
			return model.AccountState{}, err
		}
		side := model.PositionSideLong
		if qty.IsNegative() {
			side = model.PositionSideShort
			qty = qty.Abs()
		}
		state.Positions = append(state.Positions, model.PositionStatusReport{
			AccountID:    accountID,
			InstrumentID: id,
			Side:         side,
			Quantity:     qty,
			AvgPrice:     parseDecimal(p.EntryPrice),
			Unrealized:   model.Money{Amount: parseDecimal(p.UnrealizedProfit), Currency: model.Currency("USDT")},
			EventTime:    timeFromUnixMilli(p.UpdateTime),
		})
	}
	return state, nil
}

func timeFromUnixMilli(ms int64) time.Time {
	if ms <= 0 {
		return time.Now()
	}
	return time.UnixMilli(ms)
}

func moneyFromCommission(amount, currency string) model.Money {
	if currency == "" {
		return model.Money{}
	}
	return model.Money{Amount: parseDecimal(amount), Currency: model.Currency(currency)}
}

func positionSideFromQty(qty decimal.Decimal) model.PositionSide {
	switch {
	case qty.IsPositive():
		return model.PositionSideLong
	case qty.IsNegative():
		return model.PositionSideShort
	default:
		return model.PositionSideFlat
	}
}
