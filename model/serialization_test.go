package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCommandReportAndDataTypesRoundTripJSON(t *testing.T) {
	instID := MustInstrumentID("BTC-USDT-PERP.BINANCE")
	command := SubmitOrderList{
		Metadata: CommandMetadata{
			TraderID:        "trader-001",
			StrategyID:      "strategy-001",
			CommandID:       "command-001",
			CorrelationID:   "correlation-001",
			ClientID:        "binance-perp",
			ComponentID:     "strategy-engine",
			ExecAlgorithmID: "twap-001",
			ExecSpawnID:     "spawn-001",
			Params:          map[string]string{"source": "serialization-test"},
		},
		AccountID: "acct-001",
		List: OrderList{
			ID: "list-001",
			Orders: []SubmitOrder{{
				AccountID:     "acct-001",
				InstrumentID:  instID,
				OrderListID:   "list-001",
				ClientOrderID: "entry-001",
				Side:          OrderSideBuy,
				Type:          OrderTypeLimit,
				TimeInForce:   TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("101"),
			}},
		},
	}
	var decodedCommand SubmitOrderList
	roundTripJSON(t, command, &decodedCommand)
	require.NoError(t, decodedCommand.Validate())
	require.Equal(t, ExecAlgorithmID("twap-001"), decodedCommand.Metadata.ExecAlgorithmID)

	status := ExecutionMassStatus{
		Metadata:  CommandMetadata{CommandID: "mass-status-001"},
		AccountID: "acct-001",
		Venue:     "BINANCE",
		Orders: []OrderStatusReport{{
			AccountID:      "acct-001",
			InstrumentID:   instID,
			OrderID:        "order-001",
			Status:         OrderStatusFilled,
			Quantity:       decimal.RequireFromString("1"),
			FilledQuantity: decimal.RequireFromString("1"),
		}},
	}
	var decodedStatus ExecutionMassStatus
	roundTripJSON(t, status, &decodedStatus)
	require.NoError(t, decodedStatus.Validate())
	require.Equal(t, Venue("BINANCE"), decodedStatus.Venue)

	request := DataRequest{
		RequestID:    "bars-001",
		InstrumentID: instID,
		Type:         MarketDataTypeBar,
		BarType:      NewTimeBarType(instID, time.Minute),
		Start:        time.Unix(100, 0),
		End:          time.Unix(200, 0),
		Limit:        10,
	}
	var decodedRequest DataRequest
	roundTripJSON(t, request, &decodedRequest)
	require.NoError(t, decodedRequest.Validate())
	require.Equal(t, MarketDataTypeBar, decodedRequest.Type)

	event := MarketEvent{Custom: &CustomData{
		InstrumentID: instID,
		Type:         "funding_rate",
		Fields:       map[string]string{"rate": "0.0001"},
		Timestamp:    time.Unix(100, 0),
	}}
	var decodedEvent MarketEvent
	roundTripJSON(t, event, &decodedEvent)
	require.NoError(t, decodedEvent.Validate())
	require.Equal(t, "0.0001", decodedEvent.Custom.Fields["rate"])
}

func roundTripJSON[T any](t *testing.T, value T, out *T) {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, out))
}
