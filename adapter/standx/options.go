package standx

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("STANDX")

type Options struct {
	PrivateKey string
	AccountID  model.AccountID
}
