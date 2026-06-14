package nado

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("NADO")

type Options struct {
	PrivateKey string
	Subaccount string
	Sender     string
	AccountID  model.AccountID
}
