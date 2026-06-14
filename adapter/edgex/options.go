package edgex

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("EDGEX")

type Options struct {
	StarkPrivateKey   string
	ExchangeAccountID string
	AccountID         model.AccountID
}
