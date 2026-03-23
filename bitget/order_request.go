package bitget

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/shopspring/decimal"
)

func toPlaceOrderRequest(ctx context.Context, adp exchanges.Exchange, category string, params *exchanges.OrderParams) (*sdk.PlaceOrderRequest, error) {
	qty := params.Quantity
	if category == categorySpot && params.Type == exchanges.OrderTypeMarket && params.Side == exchanges.OrderSideBuy {
		refPrice := decimal.Zero
		if ob, err := adp.FetchOrderBook(ctx, params.Symbol, 1); err == nil {
			if len(ob.Asks) > 0 && ob.Asks[0].Price.IsPositive() {
				refPrice = ob.Asks[0].Price
			} else if len(ob.Bids) > 0 && ob.Bids[0].Price.IsPositive() {
				refPrice = ob.Bids[0].Price
			}
		}
		if refPrice.IsZero() {
			ticker, err := adp.FetchTicker(ctx, params.Symbol)
			if err != nil {
				return nil, err
			}
			refPrice = ticker.Ask
			if refPrice.IsZero() {
				refPrice = ticker.LastPrice
			}
		}
		if refPrice.IsZero() {
			return nil, errors.New("bitget: unable to derive spot market-buy reference price")
		}
		qty = params.Quantity.Mul(refPrice)
		qty = roundSpotMarketBuyQuoteQty(ctx, adp, params.Symbol, qty)
	}

	clientID := params.ClientID
	if clientID == "" {
		clientID = exchanges.GenerateID()
	}

	req := &sdk.PlaceOrderRequest{
		Category:   category,
		Symbol:     adp.FormatSymbol(params.Symbol),
		Qty:        qty.String(),
		Side:       strings.ToLower(string(params.Side)),
		OrderType:  strings.ToLower(string(params.Type)),
		ClientOID:  clientID,
		ReduceOnly: yesNo(params.ReduceOnly),
	}
	if params.Type == exchanges.OrderTypeLimit || params.Type == exchanges.OrderTypePostOnly {
		req.Price = params.Price.String()
	}
	if tif := toBitgetTimeInForce(params); tif != "" {
		req.TimeInForce = tif
	}
	return req, nil
}

func toModifyOrderRequest(adp exchanges.Exchange, category, orderID, symbol string, params *exchanges.ModifyOrderParams) *sdk.ModifyOrderRequest {
	req := &sdk.ModifyOrderRequest{
		Category: category,
		Symbol:   adp.FormatSymbol(symbol),
		OrderID:  orderID,
	}
	if params.Price.IsPositive() {
		req.NewPrice = params.Price.String()
	}
	if params.Quantity.IsPositive() {
		req.NewQty = params.Quantity.String()
	}
	if req.NewPrice != "" && req.NewQty != "" {
		req.NewClientID = exchanges.GenerateID()
	}
	return req
}

func toBitgetTimeInForce(params *exchanges.OrderParams) string {
	if params.Type == exchanges.OrderTypePostOnly {
		return "post_only"
	}
	switch params.TimeInForce {
	case exchanges.TimeInForceIOC:
		return "ioc"
	case exchanges.TimeInForceFOK:
		return "fok"
	case exchanges.TimeInForcePO:
		return "post_only"
	case "", exchanges.TimeInForceGTC:
		if params.Type == exchanges.OrderTypeLimit {
			return "gtc"
		}
	}
	return ""
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func mapOrder(symbol string, raw sdk.OrderRecord) *exchanges.Order {
	qty := parseDecimal(firstNonEmpty(raw.Qty, raw.BaseVolume, raw.Amount))
	filledQty := parseDecimal(firstNonEmpty(raw.FilledQty, raw.FilledVolume, raw.CumExecQty))
	price := parseDecimal(firstNonEmpty(raw.Price, raw.AvgPrice))
	ts := parseMillis(firstNonEmpty(raw.UpdatedTime, raw.CreatedTime))
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	return &exchanges.Order{
		OrderID:        raw.OrderID,
		ClientOrderID:  raw.ClientOID,
		Symbol:         symbol,
		Side:           mapOrderSide(raw.Side),
		Type:           mapOrderType(raw.OrderType, raw.TimeInForce),
		Quantity:       qty,
		Price:          price,
		Status:         mapOrderStatus(raw.OrderStatus),
		FilledQuantity: filledQty,
		Timestamp:      ts,
		Fee:            parseDecimal(firstNonEmpty(raw.Fee, feeFromDetails(raw.FeeDetail))),
		ReduceOnly:     strings.EqualFold(raw.ReduceOnly, "yes"),
		TimeInForce:    mapTimeInForce(raw.TimeInForce),
	}
}

func mapPosition(raw sdk.PositionRecord) exchanges.Position {
	side := mapPositionSide(firstNonEmpty(raw.PosSide, raw.HoldSide))
	entry := parseDecimal(firstNonEmpty(raw.AverageOpenPrice, raw.OpenPriceAvg, raw.AvgPrice))
	qty := parseDecimal(firstNonEmpty(raw.Qty, raw.Total, raw.Size))
	if side == exchanges.PositionSideShort && qty.IsPositive() {
		qty = qty.Abs()
	}
	return exchanges.Position{
		Symbol:           raw.Symbol,
		Side:             side,
		Quantity:         qty,
		EntryPrice:       entry,
		UnrealizedPnL:    parseDecimal(raw.UnrealizedPL),
		RealizedPnL:      parseDecimal(firstNonEmpty(raw.AchievedProfits, raw.CurRealisedPnl)),
		LiquidationPrice: parseDecimal(firstNonEmpty(raw.LiquidationPrice, raw.LiqPrice)),
		Leverage:         parseDecimal(raw.Leverage),
		MarginType:       strings.ToUpper(raw.MarginMode),
	}
}

func mapSpotBalances(raw []sdk.AccountAsset) []exchanges.SpotBalance {
	out := make([]exchanges.SpotBalance, 0, len(raw))
	for _, asset := range raw {
		free := parseDecimal(rawAvailable(asset))
		locked := parseDecimal(firstNonEmpty(asset.Frozen, asset.Locked))
		total := parseDecimal(firstNonEmpty(asset.Equity, free.Add(locked).String()))
		out = append(out, exchanges.SpotBalance{
			Asset:  strings.ToUpper(asset.Coin),
			Free:   free,
			Locked: locked,
			Total:  total,
		})
	}
	return out
}

func rawAvailable(asset sdk.AccountAsset) string {
	return firstNonEmpty(asset.Available, asset.Equity)
}

func mapOrderSide(side string) exchanges.OrderSide {
	if strings.EqualFold(side, "sell") {
		return exchanges.OrderSideSell
	}
	return exchanges.OrderSideBuy
}

func mapOrderType(orderType, tif string) exchanges.OrderType {
	switch strings.ToLower(orderType) {
	case "market":
		return exchanges.OrderTypeMarket
	case "limit":
		if strings.EqualFold(tif, "post_only") {
			return exchanges.OrderTypePostOnly
		}
		return exchanges.OrderTypeLimit
	default:
		return exchanges.OrderTypeUnknown
	}
}

func mapTimeInForce(tif string) exchanges.TimeInForce {
	switch strings.ToLower(tif) {
	case "ioc":
		return exchanges.TimeInForceIOC
	case "fok":
		return exchanges.TimeInForceFOK
	case "post_only":
		return exchanges.TimeInForcePO
	case "gtc":
		return exchanges.TimeInForceGTC
	default:
		return ""
	}
}

func mapOrderStatus(status string) exchanges.OrderStatus {
	switch strings.ToLower(status) {
	case "new", "init", "live":
		return exchanges.OrderStatusNew
	case "partially_filled", "partial-fill", "partially-filled":
		return exchanges.OrderStatusPartiallyFilled
	case "filled", "full_fill", "full-fill":
		return exchanges.OrderStatusFilled
	case "cancelled", "canceled":
		return exchanges.OrderStatusCancelled
	case "rejected", "fail":
		return exchanges.OrderStatusRejected
	default:
		return exchanges.OrderStatusUnknown
	}
}

func mapPositionSide(side string) exchanges.PositionSide {
	switch strings.ToLower(side) {
	case "short":
		return exchanges.PositionSideShort
	case "long":
		return exchanges.PositionSideLong
	default:
		return exchanges.PositionSideBoth
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func isBitgetOrderNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, exchanges.ErrOrderNotFound) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "30001") ||
		strings.Contains(lower, "40109") ||
		strings.Contains(lower, "doesn't exist") ||
		strings.Contains(lower, "cannot be found") ||
		strings.Contains(lower, "not found")
}

func feeFromDetails(fees []sdk.FeeDetail) string {
	if len(fees) == 0 {
		return ""
	}
	return fees[0].Fee
}

func roundSpotMarketBuyQuoteQty(ctx context.Context, adp exchanges.Exchange, symbol string, qty decimal.Decimal) decimal.Decimal {
	var minNotional decimal.Decimal
	if detail, err := adp.FetchSymbolDetails(ctx, symbol); err == nil && detail != nil && detail.MinNotional.IsPositive() && qty.LessThan(detail.MinNotional) {
		minNotional = detail.MinNotional
		qty = detail.MinNotional
	} else if detail != nil {
		minNotional = detail.MinNotional
	}

	if isClassicBitgetSpot(adp) && minNotional.IsPositive() {
		safetyFloor := minNotional.Mul(decimal.NewFromInt(2))
		if qty.LessThan(safetyFloor) {
			qty = safetyFloor
		}
	}

	precision, ok := bitgetSpotQuotePrecision(adp, symbol)
	if !ok || precision < 0 {
		if minNotional.IsPositive() && !qty.GreaterThan(minNotional) {
			return minNotional.Mul(decimal.NewFromInt(2))
		}
		return qty
	}

	step := decimal.New(1, -int32(precision))
	if step.IsZero() {
		if minNotional.IsPositive() && !qty.GreaterThan(minNotional) {
			return minNotional.Mul(decimal.NewFromInt(2))
		}
		return qty
	}
	qty = qty.Div(step).Ceil().Mul(step)
	if minNotional.IsPositive() && !qty.GreaterThan(minNotional) {
		qty = minNotional.Mul(decimal.NewFromInt(2)).Div(step).Ceil().Mul(step)
	}
	return qty
}

func bitgetSpotQuotePrecision(adp exchanges.Exchange, symbol string) (int, bool) {
	spot, ok := adp.(*SpotAdapter)
	if !ok {
		return 0, false
	}
	spot.mu.RLock()
	inst, ok := spot.markets.spotByBase[strings.ToUpper(symbol)]
	spot.mu.RUnlock()
	if !ok || inst.QuotePrecision == "" {
		return 0, false
	}
	precision, err := strconv.Atoi(inst.QuotePrecision)
	if err != nil {
		return 0, false
	}
	return precision, true
}

func isClassicBitgetSpot(adp exchanges.Exchange) bool {
	spot, ok := adp.(*SpotAdapter)
	return ok && spot.accountMode == accountModeClassic
}
