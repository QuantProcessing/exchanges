package hyperliquid

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("HYPERLIQUID")

type Options struct {
	PrivateKey     string
	Vault          string
	AccountAddress string
	AccountID      model.AccountID
}
