package okx

import (
	"context"
	"fmt"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/okx/sdk"
)

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// OKX supports native market orders; LIMIT+IOC conversion can hit OKX price-band limits.
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return nil, err
		}
	}

	instId := a.FormatSymbol(params.Symbol)

	side := "buy"
	if params.Side == exchanges.OrderSideSell {
		side = "sell"
	}

	ordType := a.mapOrderType(params)

	sz, err := a.formatPerpOrderSize(ctx, instId, params.Quantity)
	if err != nil {
		return nil, err
	}

	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()

	var clOrdId *string
	if params.ClientID != "" {
		clOrdId = &params.ClientID
	}

	a.mu.RLock()
	pm := a.posMode
	a.mu.RUnlock()

	var posSide *string
	if pm == "long_short_mode" {
		val := "long"
		if params.Side == exchanges.OrderSideBuy {
			if params.ReduceOnly {
				val = "short"
			} else {
				val = "long"
			}
		} else if params.ReduceOnly {
			val = "long"
		} else {
			val = "short"
		}
		posSide = &val
	}

	var px *string
	if params.Price.IsPositive() {
		if ok {
			prec := exchanges.CountDecimalPlaces(inst.TickSz)
			s := params.Price.StringFixed(prec)
			px = &s
		} else {
			s := fmt.Sprintf("%v", params.Price)
			px = &s
		}
	}

	req := &okx.OrderRequest{
		InstId:  instId,
		TdMode:  "isolated",
		Side:    side,
		PosSide: posSide,
		OrdType: ordType,
		Sz:      sz,
		Px:      px,
		ClOrdId: clOrdId,
	}

	if params.ReduceOnly {
		ro := true
		req.ReduceOnly = &ro
	}

	ids, err := a.client.PlaceOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no response")
	}
	if err := okxOrderActionError("place order", ids[0]); err != nil {
		return nil, err
	}
	return &exchanges.Order{
		OrderID:       ids[0].OrdId,
		ClientOrderID: ids[0].ClOrdId,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}, nil
}

func (a *Adapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	if strings.TrimSpace(params.ClientID) == "" {
		return fmt.Errorf("client id required for PlaceOrderWS")
	}
	instId := a.FormatSymbol(params.Symbol)
	side := "buy"
	if params.Side == exchanges.OrderSideSell {
		side = "sell"
	}
	ordType := a.mapOrderType(params)
	sz, err := a.formatPerpOrderSize(ctx, instId, params.Quantity)
	if err != nil {
		return err
	}

	a.mu.RLock()
	pm := a.posMode
	a.mu.RUnlock()

	var posSide *string
	if pm == "long_short_mode" {
		val := "long"
		if params.Side == exchanges.OrderSideBuy {
			if params.ReduceOnly {
				val = "short"
			} else {
				val = "long"
			}
		} else if params.ReduceOnly {
			val = "long"
		} else {
			val = "short"
		}
		posSide = &val
	}

	var clOrdId *string
	if params.ClientID != "" {
		clOrdId = &params.ClientID
	}

	var px *string
	if params.Type != exchanges.OrderTypeMarket && params.Price.IsPositive() {
		if inst, ok := a.instruments[instId]; ok {
			prec := exchanges.CountDecimalPlaces(inst.TickSz)
			s := params.Price.StringFixed(prec)
			px = &s
		} else {
			s := fmt.Sprintf("%v", params.Price)
			px = &s
		}
	}

	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("missing instrument metadata for %s", instId)
	}
	instIdCode, err := okxInstrumentIDCode(inst, instId)
	if err != nil {
		return err
	}

	req := &okx.OrderRequest{
		InstId:     instId,
		InstIdCode: &instIdCode,
		TdMode:     "isolated",
		Side:       side,
		PosSide:    posSide,
		OrdType:    ordType,
		Sz:         sz,
		Px:         px,
		ClOrdId:    clOrdId,
	}

	if params.ReduceOnly {
		ro := true
		req.ReduceOnly = &ro
	}

	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	_, err = a.wsPrivate.PlaceOrderWS(req)
	return err
}

func (a *Adapter) mapOrderType(params *exchanges.OrderParams) string {
	switch params.Type {
	case exchanges.OrderTypeMarket:
		return "market"
	case exchanges.OrderTypePostOnly:
		return "post_only"
	case exchanges.OrderTypeLimit:
		if params.TimeInForce == exchanges.TimeInForceIOC {
			return "ioc"
		} else if params.TimeInForce == exchanges.TimeInForceFOK {
			return "fok"
		}
		return "limit"
	default:
		return "limit"
	}
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	instId := a.FormatSymbol(symbol)
	resp, err := a.client.CancelOrder(ctx, instId, orderID, "")
	if err != nil {
		return err
	}
	if len(resp) == 0 {
		return nil
	}
	return okxOrderActionError("cancel order", resp[0])
}

func (a *Adapter) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	instId := a.FormatSymbol(symbol)
	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("missing instrument metadata for %s", instId)
	}
	instIdCode, err := okxInstrumentIDCode(inst, instId)
	if err != nil {
		return err
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	_, err = a.wsPrivate.CancelOrderWS(instIdCode, &orderID, nil)
	return err
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	instId := a.FormatSymbol(symbol)

	req := &okx.ModifyOrderRequest{
		InstId: instId,
		OrdId:  &orderID,
	}
	if params.Quantity.IsPositive() {
		sz := fmt.Sprintf("%v", params.Quantity)
		req.NewSz = &sz
	}
	if params.Price.IsPositive() {
		px := fmt.Sprintf("%v", params.Price)
		req.NewPx = &px
	}

	resp, err := a.client.ModifyOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("no response")
	}
	if err := okxOrderActionError("modify order", resp[0]); err != nil {
		return nil, err
	}

	return &exchanges.Order{
		OrderID:       resp[0].OrdId,
		ClientOrderID: resp[0].ClOrdId,
		Symbol:        symbol,
		Status:        exchanges.OrderStatusPending,
	}, nil
}

func (a *Adapter) ModifyOrderWS(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) error {
	instId := a.FormatSymbol(symbol)

	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("missing instrument metadata for %s", instId)
	}
	instIdCode, err := okxInstrumentIDCode(inst, instId)
	if err != nil {
		return err
	}

	req := &okx.ModifyOrderRequest{
		InstId:     instId,
		InstIdCode: &instIdCode,
		OrdId:      &orderID,
	}
	if params.Quantity.IsPositive() {
		sz := fmt.Sprintf("%v", params.Quantity)
		req.NewSz = &sz
	}
	if params.Price.IsPositive() {
		px := fmt.Sprintf("%v", params.Price)
		req.NewPx = &px
	}

	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	resp, err := a.wsPrivate.ModifyOrderWS(req)
	if err != nil {
		return err
	}
	return okxOrderActionError("modify order", *resp)
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	instId := a.FormatSymbol(symbol)

	res, err := a.client.GetOrder(ctx, instId, orderID, "")
	if err != nil {
		if isOKXOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	if len(res) == 0 {
		return nil, exchanges.ErrOrderNotFound
	}

	return a.mapOrderRest(&res[0]), nil
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	instId := a.FormatSymbol(symbol)

	var ids *string
	if instId != "" {
		ids = &instId
	}
	res, err := a.client.GetOrders(ctx, nil, ids)
	if err != nil {
		return nil, err
	}

	var orders []exchanges.Order
	for _, o := range res {
		orders = append(orders, *a.mapOrderRest(&o))
	}
	return orders, nil
}

func isOKXOrderLookupMiss(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "order") &&
		(strings.Contains(msg, "not exist") || strings.Contains(msg, "not found"))
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
	orders, err := a.FetchOpenOrders(ctx, symbol)
	if err != nil {
		return err
	}
	if len(orders) == 0 {
		return nil
	}

	instId := a.FormatSymbol(symbol)
	var reqs []okx.CancelOrderRequest
	for _, o := range orders {
		oid := o.OrderID
		reqs = append(reqs, okx.CancelOrderRequest{InstId: instId, OrdId: &oid})
	}

	chunkSize := 20
	for i := 0; i < len(reqs); i += chunkSize {
		end := i + chunkSize
		if end > len(reqs) {
			end = len(reqs)
		}
		resp, err := a.client.CancelOrders(ctx, reqs[i:end])
		if err != nil {
			return err
		}
		for _, result := range resp {
			if err := okxOrderActionError("cancel order", result); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// OKX supports native market orders; LIMIT+IOC conversion can hit OKX price-band limits.
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return nil, err
		}
	}

	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}
	instId := a.FormatSymbol(params.Symbol)

	side := "buy"
	if params.Side == exchanges.OrderSideSell {
		side = "sell"
	}

	ordType := a.mapOrderType(params)
	sz := fmt.Sprintf("%v", params.Quantity)

	var clOrdId *string
	if params.ClientID != "" {
		clOrdId = &params.ClientID
	}

	var px *string
	if params.Price.IsPositive() {
		a.mu.RLock()
		inst, ok := a.instruments[instId]
		a.mu.RUnlock()

		if ok {
			prec := exchanges.CountDecimalPlaces(inst.TickSz)
			s := params.Price.StringFixed(prec)
			px = &s
		} else {
			s := fmt.Sprintf("%v", params.Price)
			px = &s
		}
	}

	ccy := a.quoteCurrency
	var tgtCcy *string
	if ordType == "market" {
		t := "base_ccy"
		tgtCcy = &t
	}

	req := &okx.OrderRequest{
		InstId:  instId,
		TdMode:  "cash",
		Side:    side,
		PosSide: nil,
		OrdType: ordType,
		Sz:      sz,
		Px:      px,
		ClOrdId: clOrdId,
		Ccy:     &ccy,
		TgtCcy:  tgtCcy,
	}

	resp, err := a.client.PlaceOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("no response")
	}
	if err := okxOrderActionError("place order", resp[0]); err != nil {
		return nil, err
	}

	return &exchanges.Order{
		OrderID:       resp[0].OrdId,
		ClientOrderID: resp[0].ClOrdId,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}, nil
}

func (a *SpotAdapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	if strings.TrimSpace(params.ClientID) == "" {
		return fmt.Errorf("client id required for PlaceOrderWS")
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	instId := a.FormatSymbol(params.Symbol)

	side := "buy"
	if params.Side == exchanges.OrderSideSell {
		side = "sell"
	}

	ordType := a.mapOrderType(params)
	sz := fmt.Sprintf("%v", params.Quantity)

	var clOrdId *string
	if params.ClientID != "" {
		clOrdId = &params.ClientID
	}

	var px *string
	if params.Price.IsPositive() {
		a.mu.RLock()
		inst, ok := a.instruments[instId]
		a.mu.RUnlock()

		if ok {
			prec := exchanges.CountDecimalPlaces(inst.TickSz)
			s := params.Price.StringFixed(prec)
			px = &s
		} else {
			s := fmt.Sprintf("%v", params.Price)
			px = &s
		}
	}

	ccy := a.quoteCurrency
	var tgtCcy *string
	if ordType == "market" {
		t := "base_ccy"
		tgtCcy = &t
	}

	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("missing instrument metadata for %s", instId)
	}
	instIdCode, err := okxInstrumentIDCode(inst, instId)
	if err != nil {
		return err
	}

	req := &okx.OrderRequest{
		InstId:     instId,
		InstIdCode: &instIdCode,
		TdMode:     "cash",
		Side:       side,
		PosSide:    nil,
		OrdType:    ordType,
		Sz:         sz,
		Px:         px,
		ClOrdId:    clOrdId,
		Ccy:        &ccy,
		TgtCcy:     tgtCcy,
	}

	_, err = a.wsPrivate.PlaceOrderWS(req)
	return err
}

func (a *SpotAdapter) mapOrderType(params *exchanges.OrderParams) string {
	switch params.Type {
	case exchanges.OrderTypeMarket:
		return "market"
	case exchanges.OrderTypePostOnly:
		return "post_only"
	case exchanges.OrderTypeLimit:
		if params.TimeInForce == exchanges.TimeInForceIOC {
			return "ioc"
		} else if params.TimeInForce == exchanges.TimeInForceFOK {
			return "fok"
		}
		return "limit"
	default:
		return "limit"
	}
}

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	instId := a.FormatSymbol(symbol)
	resp, err := a.client.CancelOrder(ctx, instId, orderID, "")
	if err != nil {
		return err
	}
	if len(resp) == 0 {
		return nil
	}
	return okxOrderActionError("cancel order", resp[0])
}

func (a *SpotAdapter) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	instId := a.FormatSymbol(symbol)
	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("missing instrument metadata for %s", instId)
	}
	instIdCode, err := okxInstrumentIDCode(inst, instId)
	if err != nil {
		return err
	}
	_, err = a.wsPrivate.CancelOrderWS(instIdCode, &orderID, nil)
	return err
}

func (a *SpotAdapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	instId := a.FormatSymbol(symbol)

	req := &okx.ModifyOrderRequest{
		InstId: instId,
		OrdId:  &orderID,
	}
	if params.Quantity.IsPositive() {
		sz := fmt.Sprintf("%v", params.Quantity)
		req.NewSz = &sz
	}
	if params.Price.IsPositive() {
		px := fmt.Sprintf("%v", params.Price)
		req.NewPx = &px
	}

	resp, err := a.client.ModifyOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("no response")
	}
	if err := okxOrderActionError("modify order", resp[0]); err != nil {
		return nil, err
	}

	return &exchanges.Order{
		OrderID:       resp[0].OrdId,
		ClientOrderID: resp[0].ClOrdId,
		Symbol:        symbol,
		Status:        exchanges.OrderStatusPending,
	}, nil
}

func (a *SpotAdapter) ModifyOrderWS(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	instId := a.FormatSymbol(symbol)

	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("missing instrument metadata for %s", instId)
	}
	instIdCode, err := okxInstrumentIDCode(inst, instId)
	if err != nil {
		return err
	}

	req := &okx.ModifyOrderRequest{
		InstId:     instId,
		InstIdCode: &instIdCode,
		OrdId:      &orderID,
	}
	if params.Quantity.IsPositive() {
		sz := fmt.Sprintf("%v", params.Quantity)
		req.NewSz = &sz
	}
	if params.Price.IsPositive() {
		px := fmt.Sprintf("%v", params.Price)
		req.NewPx = &px
	}

	resp, err := a.wsPrivate.ModifyOrderWS(req)
	if err != nil {
		return err
	}
	return okxOrderActionError("modify order", *resp)
}

func (a *SpotAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	instId := a.FormatSymbol(symbol)

	res, err := a.client.GetOrder(ctx, instId, orderID, "")
	if err != nil {
		if isOKXOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	if len(res) == 0 {
		return nil, exchanges.ErrOrderNotFound
	}

	return a.mapOrderRest(&res[0]), nil
}

func (a *SpotAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	instId := a.FormatSymbol(symbol)

	var ids *string
	if instId != "" {
		ids = &instId
	}
	res, err := a.client.GetOrders(ctx, nil, ids)
	if err != nil {
		return nil, err
	}

	var orders []exchanges.Order
	for _, o := range res {
		orders = append(orders, *a.mapOrderRest(&o))
	}
	return orders, nil
}

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	orders, err := a.FetchOpenOrders(ctx, symbol)
	if err != nil {
		return err
	}

	instId := a.FormatSymbol(symbol)
	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("missing instrument metadata for %s", instId)
	}
	instIdCode, err := okxInstrumentIDCode(inst, instId)
	if err != nil {
		return err
	}
	var reqs []okx.CancelOrderRequest
	for _, o := range orders {
		oid := o.OrderID
		reqs = append(reqs, okx.CancelOrderRequest{InstId: instId, InstIdCode: &instIdCode, OrdId: &oid})
	}

	chunkSize := 20
	for i := 0; i < len(reqs); i += chunkSize {
		end := i + chunkSize
		if end > len(reqs) {
			end = len(reqs)
		}
		_, err := a.wsPrivate.CancelOrdersWS(reqs[i:end])
		if err != nil {
			return err
		}
	}
	return nil
}
