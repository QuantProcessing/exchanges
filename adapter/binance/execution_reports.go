package binance

import (
	"strconv"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

func spotOrderReport(accountID model.AccountID, id model.InstrumentID, resp *spot.OrderResponse) model.OrderStatusReport {
	if resp == nil {
		return model.OrderStatusReport{AccountID: accountID, InstrumentID: id}
	}
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		ClientID:     model.ClientOrderID(resp.ClientOrderID),
		Status:       orderStatusFromBinance(resp.Status),
		Side:         orderSideFromBinance(resp.Side),
		Type:         orderTypeFromBinance(resp.Type),
		Quantity:     parseDecimal(resp.OrigQty),
		FilledQty:    parseDecimal(resp.ExecutedQty),
		AvgPrice:     parseDecimal(resp.Price),
		EventTime:    timeFromUnixMilli(resp.TransactTime),
	}
}

func spotCancelReport(accountID model.AccountID, id model.InstrumentID, resp *spot.CancelOrderResponse) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		ClientID:     model.ClientOrderID(resp.ClientOrderID),
		Status:       orderStatusFromBinance(resp.Status),
		Side:         orderSideFromBinance(resp.Side),
		Type:         orderTypeFromBinance(resp.Type),
		Quantity:     parseDecimal(resp.OrigQty),
		FilledQty:    parseDecimal(resp.ExecutedQty),
		AvgPrice:     parseDecimal(resp.Price),
		EventTime:    time.Now(),
	}
}

func perpOrderReport(accountID model.AccountID, id model.InstrumentID, resp *perp.OrderResponse) model.OrderStatusReport {
	if resp == nil {
		return model.OrderStatusReport{AccountID: accountID, InstrumentID: id}
	}
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		ClientID:     model.ClientOrderID(resp.ClientOrderID),
		Status:       orderStatusFromBinance(resp.Status),
		Side:         orderSideFromBinance(resp.Side),
		Type:         orderTypeFromBinance(resp.Type),
		Quantity:     parseDecimal(resp.OrigQty),
		FilledQty:    parseDecimal(resp.ExecutedQty),
		AvgPrice:     parseDecimal(resp.AvgPrice),
		EventTime:    timeFromUnixMilli(resp.UpdateTime),
	}
}

func spotFillReport(accountID model.AccountID, id model.InstrumentID, trade spot.Trade) model.FillReport {
	side := model.OrderSideSell
	if trade.IsBuyer {
		side = model.OrderSideBuy
	}
	return model.FillReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(trade.OrderID, 10)),
		TradeID:      model.TradeID(strconv.FormatInt(trade.ID, 10)),
		Side:         side,
		Quantity:     parseDecimal(trade.Qty),
		Price:        parseDecimal(trade.Price),
		Fee:          moneyFromCommission(trade.Commission, trade.CommissionAsset),
		EventTime:    timeFromUnixMilli(trade.Time),
	}
}

func perpFillReport(accountID model.AccountID, id model.InstrumentID, trade perp.Trade) model.FillReport {
	side := model.OrderSideSell
	if trade.IsBuyer {
		side = model.OrderSideBuy
	}
	return model.FillReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(trade.OrderID, 10)),
		TradeID:      model.TradeID(strconv.FormatInt(trade.ID, 10)),
		Side:         side,
		Quantity:     parseDecimal(trade.Qty),
		Price:        parseDecimal(trade.Price),
		Fee:          moneyFromCommission(trade.Commission, trade.CommissionAsset),
		EventTime:    timeFromUnixMilli(trade.Time),
	}
}

func spotOrderReportFromStream(accountID model.AccountID, id model.InstrumentID, e *spot.ExecutionReportEvent) model.OrderStatusReport {
	if e == nil {
		return model.OrderStatusReport{AccountID: accountID, InstrumentID: id}
	}
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(e.OrderID, 10)),
		ClientID:     model.ClientOrderID(e.ClientOrderID),
		Status:       orderStatusFromBinance(e.OrderStatus),
		Side:         orderSideFromBinance(e.Side),
		Type:         orderTypeFromBinance(e.OrderType),
		Quantity:     parseDecimal(e.Quantity),
		FilledQty:    parseDecimal(e.CumulativeFilledQuantity),
		AvgPrice:     averagePrice(parseDecimal(e.CumulativeQuoteAssetTransactedQuantity), parseDecimal(e.CumulativeFilledQuantity), parseDecimal(e.LastExecutedPrice)),
		EventTime:    timeFromUnixMilli(e.EventTime),
	}
}

func spotOrderEventFromStream(accountID model.AccountID, id model.InstrumentID, e *spot.ExecutionReportEvent) model.OrderEvent {
	report := spotOrderReportFromStream(accountID, id, e)
	eventID := string(report.OrderID) + ":" + string(report.Status)
	if e != nil {
		eventID = strconv.FormatInt(e.OrderID, 10) + ":" + e.OrderStatus + ":" + strconv.FormatInt(e.TradeID, 10)
	}
	return model.OrderEvent{
		EventID:      eventID,
		AccountID:    report.AccountID,
		InstrumentID: report.InstrumentID,
		OrderID:      report.OrderID,
		ClientID:     report.ClientID,
		Type:         orderEventTypeFromStatus(report.Status),
		Status:       report.Status,
		Side:         report.Side,
		OrderType:    report.Type,
		Quantity:     report.Quantity,
		FilledQty:    report.FilledQty,
		AvgPrice:     report.AvgPrice,
		EventTime:    report.EventTime,
	}
}

func spotFillReportFromStream(accountID model.AccountID, id model.InstrumentID, e *spot.ExecutionReportEvent) (model.FillReport, bool) {
	if e == nil || e.TradeID <= 0 || parseDecimal(e.LastExecutedQuantity).IsZero() {
		return model.FillReport{}, false
	}
	return model.FillReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(e.OrderID, 10)),
		ClientID:     model.ClientOrderID(e.ClientOrderID),
		TradeID:      model.TradeID(strconv.FormatInt(e.TradeID, 10)),
		Side:         orderSideFromBinance(e.Side),
		Quantity:     parseDecimal(e.LastExecutedQuantity),
		Price:        parseDecimal(e.LastExecutedPrice),
		Fee:          moneyFromCommission(e.CommissionAmount, e.CommissionAsset),
		EventTime:    timeFromUnixMilli(e.TransactionTime),
	}, true
}

func perpOrderReportFromStream(accountID model.AccountID, id model.InstrumentID, e *perp.OrderUpdateEvent) model.OrderStatusReport {
	if e == nil {
		return model.OrderStatusReport{AccountID: accountID, InstrumentID: id}
	}
	o := e.Order
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(o.OrderID, 10)),
		ClientID:     model.ClientOrderID(o.ClientOrderID),
		Status:       orderStatusFromBinance(o.OrderStatus),
		Side:         orderSideFromBinance(o.Side),
		Type:         orderTypeFromBinance(o.OrderType),
		Quantity:     parseDecimal(o.OriginalQty),
		FilledQty:    parseDecimal(o.AccumulatedFilledQty),
		AvgPrice:     parseDecimal(o.AveragePrice),
		EventTime:    timeFromUnixMilli(e.EventTime),
	}
}

func perpOrderEventFromStream(accountID model.AccountID, id model.InstrumentID, e *perp.OrderUpdateEvent) model.OrderEvent {
	report := perpOrderReportFromStream(accountID, id, e)
	tradeID := int64(0)
	if e != nil {
		tradeID = e.Order.TradeID
	}
	return model.OrderEvent{
		EventID:      string(report.OrderID) + ":" + string(report.Status) + ":" + strconv.FormatInt(tradeID, 10),
		AccountID:    report.AccountID,
		InstrumentID: report.InstrumentID,
		OrderID:      report.OrderID,
		ClientID:     report.ClientID,
		Type:         orderEventTypeFromStatus(report.Status),
		Status:       report.Status,
		Side:         report.Side,
		OrderType:    report.Type,
		Quantity:     report.Quantity,
		FilledQty:    report.FilledQty,
		AvgPrice:     report.AvgPrice,
		EventTime:    report.EventTime,
	}
}

func perpFillReportFromStream(accountID model.AccountID, id model.InstrumentID, e *perp.OrderUpdateEvent) (model.FillReport, bool) {
	if e == nil || e.Order.TradeID <= 0 || parseDecimal(e.Order.LastFilledQty).IsZero() {
		return model.FillReport{}, false
	}
	o := e.Order
	return model.FillReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(o.OrderID, 10)),
		ClientID:     model.ClientOrderID(o.ClientOrderID),
		TradeID:      model.TradeID(strconv.FormatInt(o.TradeID, 10)),
		Side:         orderSideFromBinance(o.Side),
		Quantity:     parseDecimal(o.LastFilledQty),
		Price:        parseDecimal(o.LastFilledPrice),
		Fee:          moneyFromCommission(o.Commission, o.CommissionAsset),
		EventTime:    timeFromUnixMilli(o.TradeTime),
	}, true
}

func perpPositionsFromAccountUpdate(accountID model.AccountID, e *perp.AccountUpdateEvent) ([]model.PositionStatusReport, error) {
	if e == nil {
		return nil, nil
	}
	n := symbolNormalizer{}
	out := make([]model.PositionStatusReport, 0, len(e.UpdateData.Positions))
	for _, p := range e.UpdateData.Positions {
		id, err := n.ToInstrumentID(p.Symbol, venue.ProductHintPerp)
		if err != nil {
			return nil, err
		}
		qty := parseDecimal(p.PositionAmount)
		out = append(out, model.PositionStatusReport{
			AccountID:    accountID,
			InstrumentID: id,
			Side:         positionSideFromQty(qty),
			Quantity:     qty.Abs(),
			AvgPrice:     parseDecimal(p.EntryPrice),
			Unrealized:   model.Money{Amount: parseDecimal(p.UnrealizedPnL), Currency: model.USDT},
			EventTime:    timeFromUnixMilli(e.EventTime),
		})
	}
	return out, nil
}

func perpPositionReportFromRisk(accountID model.AccountID, id model.InstrumentID, position perp.PositionRiskResponse) model.PositionStatusReport {
	qty := parseDecimal(position.PositionAmt)
	side := positionSideFromQty(qty)
	if position.PositionSide == "LONG" {
		side = model.PositionSideLong
	}
	if position.PositionSide == "SHORT" {
		side = model.PositionSideShort
	}
	return model.PositionStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		PositionID:   model.PositionID(id.String() + ":" + string(side)),
		Side:         side,
		Quantity:     qty.Abs(),
		AvgPrice:     parseDecimal(position.EntryPrice),
		Unrealized:   model.Money{Amount: parseDecimal(position.UnRealizedProfit), Currency: model.USDT},
		EventTime:    timeFromUnixMilli(position.UpdateTime),
	}
}

func orderStatusFromBinance(status string) model.OrderStatus {
	switch status {
	case "NEW":
		return model.OrderStatusAccepted
	case "PARTIALLY_FILLED":
		return model.OrderStatusPartiallyFilled
	case "FILLED":
		return model.OrderStatusFilled
	case "CANCELED":
		return model.OrderStatusCanceled
	case "REJECTED":
		return model.OrderStatusRejected
	case "EXPIRED":
		return model.OrderStatusExpired
	default:
		return model.OrderStatusSubmitted
	}
}

func orderEventTypeFromStatus(status model.OrderStatus) model.OrderEventType {
	switch status {
	case model.OrderStatusSubmitted:
		return model.OrderEventSubmitted
	case model.OrderStatusAccepted:
		return model.OrderEventAccepted
	case model.OrderStatusRejected:
		return model.OrderEventRejected
	case model.OrderStatusPartiallyFilled:
		return model.OrderEventPartiallyFilled
	case model.OrderStatusFilled:
		return model.OrderEventFilled
	case model.OrderStatusCanceled:
		return model.OrderEventCanceled
	case model.OrderStatusExpired:
		return model.OrderEventExpired
	default:
		return model.OrderEventSubmitted
	}
}

func orderSideFromBinance(side string) model.OrderSide {
	if side == "SELL" {
		return model.OrderSideSell
	}
	return model.OrderSideBuy
}

func orderTypeFromBinance(t string) model.OrderType {
	if t == "LIMIT" {
		return model.OrderTypeLimit
	}
	return model.OrderTypeMarket
}

func averagePrice(notional, quantity, fallback decimal.Decimal) decimal.Decimal {
	if quantity.IsZero() {
		return fallback
	}
	return notional.Div(quantity)
}
