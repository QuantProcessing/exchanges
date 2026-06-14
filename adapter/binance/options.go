package binance

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("BINANCE")

type Options struct {
	APIKey    string
	SecretKey string
	AccountID model.AccountID
}
