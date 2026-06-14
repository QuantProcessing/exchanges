package backpack

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("BACKPACK")

type Options struct {
	APIKey     string
	PrivateKey string
	AccountID  model.AccountID
}
