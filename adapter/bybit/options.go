package bybit

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("BYBIT")

type Options struct {
	APIKey    string
	SecretKey string
	AccountID model.AccountID
}
