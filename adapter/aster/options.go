package aster

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("ASTER")

type Options struct {
	APIKey    string
	SecretKey string
	AccountID model.AccountID
}
