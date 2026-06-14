package bitget

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("BITGET")

type Options struct {
	APIKey     string
	SecretKey  string
	Passphrase string
	AccountID  model.AccountID
}
