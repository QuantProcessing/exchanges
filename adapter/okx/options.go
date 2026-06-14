package okx

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("OKX")

type Options struct {
	APIKey     string
	SecretKey  string
	Passphrase string
	AccountID  model.AccountID
}
