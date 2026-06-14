package grvt

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("GRVT")

type Options struct {
	APIKey       string
	SubAccountID string
	PrivateKey   string
	AccountID    model.AccountID
}
