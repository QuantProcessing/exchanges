package backpack

import (
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
)

var backpackClientIDCounter atomic.Uint32

func init() {
	backpackClientIDCounter.Store(uint32(time.Now().UnixMilli() % math.MaxUint32))
}

// GenerateClientID returns a Backpack-compatible client order ID.
// The returned value is a base-10 numeric string in the valid uint32 range and is never "0".
func GenerateClientID() string {
	return nextBackpackClientID()
}

func ensureOrderParamsClientID(params *exchanges.OrderParams) error {
	if params == nil {
		return fmt.Errorf("order params required")
	}
	raw, _, err := ensureBackpackClientID(params.ClientID)
	if err != nil {
		return err
	}
	params.ClientID = raw
	return nil
}

func toCreateOrderRequest(market sdk.Market, params *exchanges.OrderParams) (sdk.CreateOrderRequest, error) {
	clientIDRaw, clientID, err := ensureBackpackClientID(params.ClientID)
	if err != nil {
		return sdk.CreateOrderRequest{}, err
	}
	params.ClientID = clientIDRaw

	req := sdk.CreateOrderRequest{
		Symbol:     market.Symbol,
		Side:       toSDKSide(params.Side),
		OrderType:  toSDKOrderType(params.Type),
		Quantity:   params.Quantity.String(),
		ReduceOnly: params.ReduceOnly,
		ClientID:   clientID,
	}
	if params.Type == exchanges.OrderTypeLimit || params.Price.IsPositive() {
		req.Price = params.Price.String()
	}
	if params.TimeInForce != "" {
		req.TimeInForce = toSDKTIF(params.TimeInForce)
	}
	return req, nil
}

func toCancelOrderRequest(orderID, symbol string) sdk.CancelOrderRequest {
	return sdk.CancelOrderRequest{
		OrderID: orderID,
		Symbol:  symbol,
	}
}

func toSDKSide(side exchanges.OrderSide) string {
	if side == exchanges.OrderSideSell {
		return "Ask"
	}
	return "Bid"
}

func toSDKOrderType(orderType exchanges.OrderType) string {
	switch orderType {
	case exchanges.OrderTypeLimit, exchanges.OrderTypePostOnly:
		return "Limit"
	default:
		return "Market"
	}
}

func toSDKTIF(tif exchanges.TimeInForce) string {
	switch tif {
	case exchanges.TimeInForceIOC:
		return "IOC"
	case exchanges.TimeInForceFOK:
		return "FOK"
	case exchanges.TimeInForcePO:
		return "PostOnly"
	default:
		return "GTC"
	}
}

func ensureBackpackClientID(raw string) (string, uint32, error) {
	if raw == "" {
		generated := GenerateClientID()
		parsed, err := parseClientID(generated)
		if err != nil {
			return "", 0, err
		}
		return generated, parsed, nil
	}
	parsed, err := parseClientID(raw)
	if err != nil {
		return "", 0, err
	}
	return raw, parsed, nil
}

func nextBackpackClientID() string {
	for {
		v := backpackClientIDCounter.Add(1)
		if v != 0 {
			return strconv.FormatUint(uint64(v), 10)
		}
	}
}

func parseClientID(raw string) (uint32, error) {
	v, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("backpack: invalid client id %q: %w", raw, err)
	}
	if v == 0 {
		return 0, fmt.Errorf("backpack: client id must be non-zero")
	}
	return uint32(v), nil
}
