package okx

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	sdkokx "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

func okxAccountState(accountID model.AccountID, balances []sdkokx.Balance) (model.AccountState, error) {
	state := model.AccountState{
		AccountID: accountID,
		Venue:     model.VenueOKX,
		Type:      model.AccountTypeMargin,
		Reported:  true,
		EventTime: time.Now(),
		InitTime:  time.Now(),
	}
	for _, balance := range balances {
		if ts := timeFromOKXMillis(balance.UTime); !ts.IsZero() {
			state.EventTime = ts
		}
		initial := model.Money{Amount: parseString(balance.Imr), Currency: model.USD}
		maintenance := model.Money{Amount: parseString(balance.Mmr), Currency: model.USD}
		if !initial.Amount.IsZero() || !maintenance.Amount.IsZero() {
			state.Margins = append(state.Margins, model.MarginBalance{
				Initial:     initial,
				Maintenance: maintenance,
			})
		}
		for _, detail := range balance.Details {
			accountBalance, ok, err := okxBalanceDetail(detail)
			if err != nil {
				return model.AccountState{}, err
			}
			if ok {
				state.Balances = append(state.Balances, accountBalance)
			}
		}
	}
	return state, nil
}

func okxBalanceDetail(detail sdkokx.BalanceDetail) (model.AccountBalance, bool, error) {
	ccy := model.Currency(strings.ToUpper(strings.TrimSpace(detail.Ccy)))
	if ccy == "" {
		return model.AccountBalance{}, false, nil
	}
	totalAmount := firstPositiveDecimal(detail.Eq, detail.CashBal)
	if totalAmount.IsZero() {
		return model.AccountBalance{}, false, nil
	}
	lockedAmount := parseString(detail.FrozenBal).Add(parseString(detail.OrdFrozen))
	if lockedAmount.GreaterThan(totalAmount) {
		lockedAmount = decimal.Zero
	}
	total := model.Money{Amount: totalAmount, Currency: ccy}
	locked := model.Money{Amount: lockedAmount, Currency: ccy}
	balance, err := model.BalanceFromTotalAndLocked(total, locked)
	return balance, err == nil, err
}

func okxPositionReport(accountID model.AccountID, inst model.Instrument, pos sdkokx.Position) (model.PositionStatusReport, bool, error) {
	if strings.ToUpper(pos.InstType) != "SWAP" {
		return model.PositionStatusReport{}, false, nil
	}
	rawQty := parseString(pos.Pos)
	if rawQty.IsZero() {
		return model.PositionStatusReport{}, false, nil
	}
	normalizer := symbolNormalizer{}
	id, err := normalizer.ToInstrumentID(pos.InstId, venue.ProductHintPerp)
	if err != nil {
		return model.PositionStatusReport{}, false, err
	}
	if id != inst.ID {
		return model.PositionStatusReport{}, false, nil
	}
	side := model.PositionSideLong
	switch pos.PosSide {
	case sdkokx.PosSideShort:
		side = model.PositionSideShort
	case sdkokx.PosSideNet:
		if rawQty.IsNegative() {
			side = model.PositionSideShort
			rawQty = rawQty.Abs()
		}
	}
	if inst.Multiplier.IsPositive() {
		rawQty = rawQty.Mul(inst.Multiplier)
	}
	settle := model.Currency(partAt(strings.Split(id.Symbol, "-"), 1))
	return model.PositionStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		PositionID:   model.PositionID(pos.PosId),
		Side:         side,
		Quantity:     rawQty,
		AvgPrice:     parseString(pos.AvgPx),
		Unrealized:   model.Money{Amount: parseString(pos.Upl), Currency: settle},
		EventTime:    timeFromOKXMillis(pos.UTime),
	}, true, nil
}

func firstPositiveDecimal(values ...string) decimal.Decimal {
	for _, value := range values {
		parsed := parseString(value)
		if parsed.IsPositive() {
			return parsed
		}
	}
	return decimal.Zero
}

func timeFromOKXMillis(raw string) time.Time {
	ms := parseTime(raw)
	if ms <= 0 {
		return time.Now()
	}
	return time.UnixMilli(ms)
}

func invalidOKXAccountState(msg string) error {
	return fmt.Errorf("%w: %s", model.ErrInvalidAccountState, msg)
}
