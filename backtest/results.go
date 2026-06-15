package backtest

import (
	"encoding/json"
	"sort"

	"github.com/QuantProcessing/exchanges/model"
)

type ResultSummary struct {
	EventsProcessed int                    `json:"events_processed"`
	Accounts        []AccountResultSummary `json:"accounts"`
}

type AccountResultSummary struct {
	AccountID model.AccountID              `json:"account_id"`
	Orders    []model.OrderStatusReport    `json:"orders"`
	Fills     []model.FillReport           `json:"fills"`
	Positions []model.PositionStatusReport `json:"positions"`
}

func (r Result) Summary(accountIDs ...model.AccountID) ResultSummary {
	summary := ResultSummary{EventsProcessed: r.EventsProcessed}
	if r.Cache == nil {
		return summary
	}
	ids := append([]model.AccountID(nil), accountIDs...)
	if len(ids) == 0 {
		ids = append(ids, r.AccountIDs...)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, accountID := range ids {
		orders := r.Cache.Orders(accountID)
		fills := make([]model.FillReport, 0)
		for _, order := range orders {
			fills = append(fills, r.Cache.FillsForOrder(accountID, order.OrderID)...)
		}
		sort.Slice(fills, func(i, j int) bool {
			if fills[i].Timestamp.Equal(fills[j].Timestamp) {
				if fills[i].OrderID == fills[j].OrderID {
					return fills[i].TradeID < fills[j].TradeID
				}
				return fills[i].OrderID < fills[j].OrderID
			}
			return fills[i].Timestamp.Before(fills[j].Timestamp)
		})
		summary.Accounts = append(summary.Accounts, AccountResultSummary{
			AccountID: accountID,
			Orders:    orders,
			Fills:     fills,
			Positions: r.Cache.Positions(accountID),
		})
	}
	return summary
}

func (r Result) DeterministicJSON(accountIDs ...model.AccountID) ([]byte, error) {
	return json.Marshal(r.Summary(accountIDs...))
}
