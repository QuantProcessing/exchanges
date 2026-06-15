package model

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestReportGenerationCommandsValidateExecutionScope(t *testing.T) {
	instID := MustInstrumentID("BTC-USDT-PERP.BINANCE")
	metadata := CommandMetadata{
		TraderID:      "trader-001",
		StrategyID:    "strategy-001",
		CommandID:     "command-001",
		CorrelationID: "correlation-001",
		ClientID:      "binance-perp",
	}

	orderReports := GenerateOrderStatusReports{
		Metadata:     metadata,
		AccountID:    "acct-001",
		InstrumentID: instID,
	}
	require.NoError(t, orderReports.Validate())

	fillReports := GenerateFillReports{
		Metadata:     metadata,
		AccountID:    "acct-001",
		InstrumentID: instID,
		StartTradeID: "trade-001",
	}
	require.NoError(t, fillReports.Validate())

	positionReports := GeneratePositionStatusReports{
		Metadata:        metadata,
		AccountID:       "acct-001",
		InstrumentID:    instID,
		VenuePositionID: "venue-position-001",
	}
	require.NoError(t, positionReports.Validate())

	massStatus := GenerateExecutionMassStatus{
		Metadata:        metadata,
		AccountID:       "acct-001",
		InstrumentID:    instID,
		VenueOrderID:    "venue-order-001",
		VenuePositionID: "venue-position-001",
	}
	require.NoError(t, massStatus.Validate())
	require.Equal(t, VenuePositionID("venue-position-001"), massStatus.VenuePositionID)

	massStatus.AccountID = ""
	require.ErrorIs(t, massStatus.Validate(), ErrInvalidAccount)
}

func TestSubmitOrderListAppliesCommandMetadataDefaults(t *testing.T) {
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
		},
		AccountID: "acct-001",
		List: OrderList{
			ID: "list-001",
			Orders: []SubmitOrder{
				{
					AccountID:     "acct-001",
					InstrumentID:  instID,
					OrderListID:   "list-001",
					ClientOrderID: "entry-001",
					Side:          OrderSideBuy,
					Type:          OrderTypeLimit,
					TimeInForce:   TimeInForceGTC,
					Quantity:      decimal.RequireFromString("1"),
					Price:         decimal.RequireFromString("101"),
				},
				{
					AccountID:           "acct-001",
					InstrumentID:        instID,
					OrderListID:         "list-001",
					ParentClientOrderID: "entry-001",
					ClientOrderID:       "take-profit-001",
					Side:                OrderSideSell,
					Type:                OrderTypeLimit,
					Contingency:         ContingencyTypeOUO,
					TimeInForce:         TimeInForceGTC,
					Quantity:            decimal.RequireFromString("1"),
					Price:               decimal.RequireFromString("103"),
					ReduceOnly:          true,
				},
			},
		},
	}

	withDefaults := command.WithCommandMetadataDefaults()
	require.NoError(t, withDefaults.Validate())
	require.Equal(t, ComponentID("strategy-engine"), withDefaults.List.Metadata.ComponentID)
	require.Equal(t, ExecAlgorithmID("twap-001"), withDefaults.List.Orders[0].Metadata.ExecAlgorithmID)
	require.Equal(t, ExecSpawnID("spawn-001"), withDefaults.List.Orders[1].Metadata.ExecSpawnID)
	require.Equal(t, CommandID("command-001"), withDefaults.List.Orders[0].Metadata.CommandID)
}

func TestExecutionMassStatusValidatesGroupedReports(t *testing.T) {
	instID := MustInstrumentID("BTC-USDT-PERP.BINANCE")
	status := ExecutionMassStatus{
		Metadata:  CommandMetadata{CommandID: "mass-status-001"},
		AccountID: "acct-001",
		Venue:     "BINANCE",
		Accounts: []AccountSnapshot{{
			AccountID: "acct-001",
			Venue:     "BINANCE",
			Type:      AccountTypeMargin,
		}},
		Orders: []OrderStatusReport{{
			AccountID:      "acct-001",
			InstrumentID:   instID,
			OrderID:        "order-001",
			VenueOrderID:   "venue-order-001",
			ClientOrderID:  "client-order-001",
			Status:         OrderStatusFilled,
			Side:           OrderSideBuy,
			Type:           OrderTypeLimit,
			Quantity:       decimal.RequireFromString("1"),
			FilledQuantity: decimal.RequireFromString("1"),
		}},
		Fills: []FillReport{{
			AccountID:     "acct-001",
			InstrumentID:  instID,
			OrderID:       "order-001",
			VenueOrderID:  "venue-order-001",
			ClientOrderID: "client-order-001",
			TradeID:       "trade-001",
			Side:          OrderSideBuy,
			Price:         decimal.RequireFromString("101"),
			Quantity:      decimal.RequireFromString("1"),
			FeeCurrency:   "USDT",
		}},
		Positions: []PositionStatusReport{{
			AccountID:       "acct-001",
			InstrumentID:    instID,
			PositionID:      "position-001",
			VenuePositionID: "venue-position-001",
			Side:            PositionSideFlat,
			Quantity:        decimal.Zero,
		}},
	}
	require.NoError(t, status.Validate())

	status.Orders[0].AccountID = "other-account"
	require.ErrorIs(t, status.Validate(), ErrInvalidOrder)
}
